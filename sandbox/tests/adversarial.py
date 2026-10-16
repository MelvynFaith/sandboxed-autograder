#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Functional and adversarial tests for sandbox-exec.

Run through tests/run.sh, which provides the delegated cgroup:

    tests/run.sh /abs/path/to/sandbox-exec [unittest arguments]

Each test runs a small Python program inside the sandbox and checks the JSON report.
"""
import json
import os
import platform
import shutil
import signal
import subprocess
import tempfile
import textwrap
import time
import unittest
from pathlib import Path

BIN = os.environ["SBX_BIN"]
CGROUP_ROOT = os.environ["SBX_CGROUP_ROOT"]
RUN_DIR = tempfile.mkdtemp(prefix="suite-")
X86_64 = platform.machine() == "x86_64"

DEFAULTS = dict(
    cpu_time_ms=2000,
    timeout_s=5,
    memory_mb=128,
    max_processes=16,
    max_output_bytes=65536,
    tmpfs_size_mb=32,
)


def limit_args(**limits):
    cfg = dict(DEFAULTS, **limits)
    return [
        "--cpu-time-ms", str(cfg["cpu_time_ms"]),
        "--timeout-s", str(cfg["timeout_s"]),
        "--memory-mb", str(cfg["memory_mb"]),
        "--max-processes", str(cfg["max_processes"]),
        "--max-output-bytes", str(cfg["max_output_bytes"]),
        "--tmpfs-size-mb", str(cfg["tmpfs_size_mb"]),
    ]


def base_args(code_path, **limits):
    return [
        BIN, "--cgroup-root", CGROUP_ROOT, "--run-dir", RUN_DIR,
        *limit_args(**limits),
        "--copy", f"{code_path}:main.py",
        # Not -I: it implies -E, which makes Python ignore PYTHONHASHSEED.
        "--", "/usr/bin/python3", "-s", "-P", "-B", "main.py",
    ]


class Report(dict):
    """Parsed JSON report plus the exit status and stderr of sandbox-exec itself."""

    rc = None
    diag = ""


def sandbox(code, stdin=b"", **limits):
    if isinstance(stdin, str):
        stdin = stdin.encode()
    with tempfile.TemporaryDirectory(dir=RUN_DIR) as tmp:
        path = Path(tmp) / "main.py"
        path.write_text(textwrap.dedent(code))
        proc = subprocess.run(base_args(path, **limits), input=stdin, capture_output=True, timeout=120)
    try:
        rep = Report(json.loads(proc.stdout))
    except ValueError:
        rep = Report()
    rep.rc = proc.returncode
    rep.diag = proc.stderr.decode(errors="replace")
    return rep


def sbx_entries(root):
    try:
        return sorted(n for n in os.listdir(root) if n.startswith("sbx-"))
    except FileNotFoundError:
        return []


def tearDownModule():
    # No cgroup and no run directory may survive the suite.
    leftovers = sbx_entries(CGROUP_ROOT) + sbx_entries(RUN_DIR)
    shutil.rmtree(RUN_DIR, ignore_errors=True)
    assert not leftovers, f"leftovers after the suite: {leftovers}"


class Functional(unittest.TestCase):
    def test_hello_and_report_shape(self):
        r = sandbox('print("hello")')
        self.assertEqual(r.rc, 0, r.diag)
        self.assertEqual(r["schema_version"], 1)
        self.assertEqual(r["termination_reason"], "EXITED")
        self.assertEqual(r["exit_code"], 0)
        self.assertIsNone(r["signal"])
        self.assertEqual(r["stdout"], "hello\n")
        self.assertEqual(r["stderr"], "")
        self.assertFalse(r["output_truncated"])
        self.assertIsNone(r["violation"])
        self.assertIsNone(r["error"])
        self.assertIn(r["memory_source"], ("CGROUP_PEAK", "CGROUP_SAMPLED", "RUSAGE"))
        self.assertGreater(r["memory_kb"], 0)

    def test_stdin_roundtrip(self):
        r = sandbox("import sys\nsys.stdout.write(sys.stdin.read().upper())", stdin="abc\ndef\n")
        self.assertEqual(r["stdout"], "ABC\nDEF\n")

    def test_large_stdin_that_is_never_read_does_not_deadlock(self):
        r = sandbox('print("done")', stdin=b"x" * (3 << 20))
        self.assertEqual(r["termination_reason"], "EXITED")
        self.assertEqual(r["stdout"], "done\n")

    def test_runtime_error_goes_to_stderr(self):
        r = sandbox('raise ValueError("boom")')
        self.assertEqual(r["termination_reason"], "EXITED")
        self.assertEqual(r["exit_code"], 1)
        self.assertIn("ValueError: boom", r["stderr"])

    def test_syntax_error_goes_to_stderr(self):
        r = sandbox("def f(:\n    pass\n")
        self.assertEqual(r["exit_code"], 1)
        self.assertIn("SyntaxError", r["stderr"])

    def test_exit_code_is_reported(self):
        r = sandbox("import sys\nsys.exit(3)")
        self.assertEqual((r["termination_reason"], r["exit_code"]), ("EXITED", 3))

    def test_killed_by_signal_is_signaled(self):
        r = sandbox("import os, signal\nos.kill(os.getpid(), signal.SIGSEGV)\nimport time\ntime.sleep(5)",
                    timeout_s=2)
        # PID 1 ignores a SIGSEGV sent with kill(); the wall clock ends the run instead.
        self.assertIn(r["termination_reason"], ("SIGNALED", "TIMEOUT_WALL"))

    def test_real_segfault_is_signaled(self):
        r = sandbox("import ctypes\nctypes.string_at(0)")
        self.assertEqual(r["termination_reason"], "SIGNALED")
        self.assertEqual(r["signal"], signal.SIGSEGV)
        self.assertIsNone(r["exit_code"])

    def test_invalid_utf8_output_keeps_the_report_valid(self):
        r = sandbox("import sys\nsys.stdout.buffer.write(b'a\\xff\\xfeb')")
        self.assertEqual(r["stdout"], "a��b")

    def test_legitimate_stdlib_programs_are_not_flagged(self):
        r = sandbox(
            """
            import asyncio, threading, queue, json, re, sqlite3, subprocess, sys, os, time
            async def f(x):
                await asyncio.sleep(0.01)
                return x
            print(asyncio.run(f(5)))
            q = queue.Queue()
            t = threading.Thread(target=lambda: q.put(7)); t.start(); t.join()
            print(q.get())
            db = sqlite3.connect(":memory:"); db.execute("create table t(a)"); print(db.execute("select 1").fetchall())
            print(subprocess.run([sys.executable, "-I", "-c", "print('child')"], capture_output=True, text=True).stdout.strip())
            pid = os.fork()
            if pid == 0:
                os._exit(4)
            print(os.waitpid(pid, 0)[1] >> 8)
            """
        )
        self.assertEqual(r["termination_reason"], "EXITED", r)
        self.assertEqual(r["stdout"], "5\n7\n[(1,)]\nchild\n4\n")

    def test_result_file_option(self):
        with tempfile.TemporaryDirectory(dir=RUN_DIR) as tmp:
            code = Path(tmp) / "main.py"
            code.write_text('print("x")')
            out = Path(tmp) / "report.json"
            proc = subprocess.run(base_args(code) + [], input=b"", capture_output=True)
            self.assertEqual(json.loads(proc.stdout)["stdout"], "x\n")
            args = base_args(code)
            args[1:1] = ["--result", str(out)]
            proc = subprocess.run(args, input=b"", capture_output=True)
            self.assertEqual(proc.stdout, b"")
            self.assertEqual(json.loads(out.read_text())["stdout"], "x\n")
            self.assertEqual(out.stat().st_mode & 0o777, 0o600)

    def test_copy_into_subdirectory(self):
        with tempfile.TemporaryDirectory(dir=RUN_DIR) as tmp:
            helper = Path(tmp) / "h.txt"
            helper.write_text("data")
            code = Path(tmp) / "main.py"
            code.write_text('print(open("sub/dir/h.txt").read())')
            args = base_args(code)
            args[1:1] = ["--copy", f"{helper}:sub/dir/h.txt"]
            proc = subprocess.run(args, input=b"", capture_output=True)
            self.assertEqual(json.loads(proc.stdout)["stdout"], "data\n")


class Limits(unittest.TestCase):
    def test_cpu_exhaustion_is_cut_at_the_cpu_budget(self):
        t0 = time.monotonic()
        r = sandbox("while True:\n    pass", cpu_time_ms=800, timeout_s=10)
        self.assertEqual(r["termination_reason"], "TIMEOUT_CPU")
        self.assertGreaterEqual(r["cpu_ms"], 800)
        self.assertLess(r["cpu_ms"], 1500, "the budget has millisecond precision, not whole seconds")
        self.assertLess(time.monotonic() - t0, 5)

    def test_wall_clock_without_cpu(self):
        r = sandbox("import time\ntime.sleep(3600)", timeout_s=1)
        self.assertEqual(r["termination_reason"], "TIMEOUT_WALL")
        self.assertLess(r["cpu_ms"], 500, "sleep() uses no CPU, so only the wall clock can stop it")
        self.assertGreaterEqual(r["wall_ms"], 1000)
        self.assertLess(r["wall_ms"], 2500)

    def test_memory_bomb(self):
        r = sandbox("x = []\nwhile True:\n    x.append(b'x' * (4 << 20))", memory_mb=64, tmpfs_size_mb=16, timeout_s=20)
        self.assertEqual(r["termination_reason"], "MEMORY_LIMIT", r)
        self.assertLessEqual(r["memory_kb"], 70 * 1024)

    def test_memory_bomb_cannot_escape_through_swap(self):
        # With memory.swap.max=0 the kernel kills instead of paging out; with swap this would
        # thrash until the wall clock.
        t0 = time.monotonic()
        r = sandbox("x = []\nwhile True:\n    x.append(b'y' * (16 << 20))", memory_mb=96, tmpfs_size_mb=16, timeout_s=20)
        self.assertEqual(r["termination_reason"], "MEMORY_LIMIT", r)
        self.assertLess(time.monotonic() - t0, 15)

    def test_peak_memory_covers_all_processes(self):
        r = sandbox(
            """
            import os, time
            pids = []
            for _ in range(3):
                pid = os.fork()
                if pid == 0:
                    block = b"x" * (20 << 20)
                    time.sleep(0.8)
                    os._exit(0)
                pids.append(pid)
            for p in pids:
                os.waitpid(p, 0)
            """,
            memory_mb=160,
        )
        self.assertEqual(r["termination_reason"], "EXITED", r)
        self.assertGreaterEqual(r["memory_kb"], 55 * 1024, "ru_maxrss would report one process only")
        self.assertIn(r["memory_source"], ("CGROUP_PEAK", "CGROUP_SAMPLED"))

    def test_fork_bomb(self):
        r = sandbox(
            """
            import os
            while True:
                try:
                    os.fork()
                except OSError:
                    pass
            """,
            max_processes=16, timeout_s=3, cpu_time_ms=20000,
        )
        self.assertIn(r["termination_reason"], ("TIMEOUT_WALL", "TIMEOUT_CPU"), r)
        self.assertTrue(r["pids_limit_hit"])

    def test_output_bomb_stdout(self):
        r = sandbox('while True:\n    print("x" * 4096)', max_output_bytes=100000, timeout_s=10)
        self.assertEqual(r["termination_reason"], "OUTPUT_LIMIT", r)
        self.assertTrue(r["output_truncated"])
        self.assertLessEqual(len(r["stdout"]) + len(r["stderr"]), 100000)

    def test_output_bomb_stderr_counts_too(self):
        r = sandbox('import sys\nwhile True:\n    sys.stderr.write("e" * 4096)', max_output_bytes=100000, timeout_s=10)
        self.assertEqual(r["termination_reason"], "OUTPUT_LIMIT", r)
        self.assertTrue(r["output_truncated"])

    def test_output_limit_is_combined_not_per_stream(self):
        # 60 kB per stream, 120 kB together against a 100 kB cap.
        r = sandbox(
            "import sys\nsys.stdout.write('o' * 60000)\nsys.stdout.flush()\nsys.stderr.write('e' * 60000)",
            max_output_bytes=100000,
        )
        self.assertEqual(r["termination_reason"], "OUTPUT_LIMIT", r)

    def test_output_exactly_at_the_limit_is_accepted(self):
        r = sandbox("import sys\nsys.stdout.write('a' * 100000)", max_output_bytes=100000)
        self.assertEqual(r["termination_reason"], "EXITED", r)
        self.assertFalse(r["output_truncated"])
        self.assertEqual(len(r["stdout"]), 100000)

    def test_disk_write_bomb_hits_tmpfs_before_the_memory_cgroup(self):
        r = sandbox(
            """
            import errno, os
            try:
                with open("big", "wb") as f:
                    while True:
                        f.write(b"x" * (1 << 20))
                        f.flush()
            except OSError as e:
                os.remove("big")  # stdout lives in the same tmpfs; free space to be able to report
                print("ENOSPC" if e.errno == errno.ENOSPC else e.errno)
            """,
            memory_mb=64, tmpfs_size_mb=16, timeout_s=20,
        )
        self.assertEqual(r["termination_reason"], "EXITED", r)
        self.assertEqual(r["stdout"], "ENOSPC\n")

    def test_everything_dies_with_the_sandbox(self):
        r = sandbox(
            """
            import os, time
            if os.fork() == 0:
                time.sleep(3600)
            time.sleep(3600)
            """,
            timeout_s=1,
        )
        self.assertEqual(r["termination_reason"], "TIMEOUT_WALL")
        # tearDownModule fails if the cgroup survives: it cannot be removed while tasks live.


class Concurrency(unittest.TestCase):
    def test_parallel_runs_do_not_interfere(self):
        from concurrent.futures import ThreadPoolExecutor

        def one(i):
            r = sandbox(f"import os\nopen('f','w').write('{i}')\nprint({i}, open('f').read(), os.getpid())")
            return i, r

        with ThreadPoolExecutor(8) as ex:
            results = list(ex.map(one, range(24)))
        for i, r in results:
            self.assertEqual(r["termination_reason"], "EXITED", r)
            self.assertEqual(r["stdout"], f"{i} {i} 1\n")

    def test_one_runaway_does_not_starve_the_others(self):
        from concurrent.futures import ThreadPoolExecutor

        with ThreadPoolExecutor(5) as ex:
            bad = ex.submit(sandbox, "while True:\n    pass", cpu_time_ms=3000, timeout_s=10)
            good = [ex.submit(sandbox, "print(sum(range(10000)))") for _ in range(4)]
            for g in good:
                self.assertEqual(g.result()["stdout"], "49995000\n")
            self.assertEqual(bad.result()["termination_reason"], "TIMEOUT_CPU")


class Isolation(unittest.TestCase):
    def test_path_traversal_sees_nothing_of_the_host(self):
        r = sandbox(
            """
            import os
            print(sorted(os.listdir("/")))
            print(os.listdir("/etc"))
            for p in ("/etc/passwd", "/etc/shadow", "/home", "/root", "/var", "/tmp", "/sys"):
                print(p, os.path.exists(p))
            try:
                open("/etc/passwd")
            except OSError as e:
                print(type(e).__name__)
            """
        )
        self.assertEqual(r["termination_reason"], "EXITED", r)
        lines = r["stdout"].splitlines()
        top = eval(lines[0])  # noqa: S307 - output of our own test program
        for forbidden in ("home", "root", "var", "tmp", "sys", "mnt", "opt", "srv", "boot"):
            self.assertNotIn(forbidden, top)
        self.assertIn("usr", top)
        self.assertIn("work", top)
        self.assertEqual(lines[1], "['ld.so.cache']")
        self.assertTrue(all(line.endswith("False") for line in lines[2:9]), lines)
        self.assertEqual(lines[9], "FileNotFoundError")

    def test_runs_unprivileged_with_no_capabilities(self):
        r = sandbox(
            """
            import os, socket
            st = dict(l.split(":\\t", 1) for l in open("/proc/self/status").read().splitlines() if ":\\t" in l)
            for k in ("CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb", "NoNewPrivs", "Seccomp", "Uid", "Gid"):
                print(k, st[k].strip())
            print("pid", os.getpid(), "host", socket.gethostname())
            """
        )
        out = dict(line.split(" ", 1) for line in r["stdout"].splitlines())
        zeros = "0000000000000000"
        for cap in ("CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"):
            self.assertEqual(out[cap], zeros, cap)
        self.assertEqual(out["NoNewPrivs"], "1")
        self.assertEqual(out["Seccomp"], "2")
        self.assertEqual(out["Uid"].split()[0], "65534")
        self.assertEqual(out["Gid"].split()[0], "65534")
        self.assertEqual(out["pid"], "1 host sandbox")

    def test_cannot_regain_root_or_privilege(self):
        r = sandbox(
            """
            import os
            for fn in (lambda: os.setuid(0), lambda: os.setgid(0), lambda: os.chroot("/")):
                try:
                    fn()
                    print("allowed")
                except OSError as e:
                    print("denied", e.errno)
            """
        )
        # setuid() is not on the allowlist, so the first attempt is a violation.
        self.assertEqual(r["termination_reason"], "SECURITY_VIOLATION", r)

    def test_proc_shows_only_the_sandbox(self):
        r = sandbox("import os\nprint(sorted(d for d in os.listdir('/proc') if d.isdigit()))")
        self.assertEqual(r["stdout"], "['1']\n")

    def test_root_and_usr_are_read_only(self):
        r = sandbox(
            """
            import errno
            for p in ("/x", "/usr/x", "/etc/x", "/dev/x"):
                try:
                    open(p, "w")
                    print(p, "writable")
                except OSError as e:
                    print(p, errno.errorcode[e.errno])
            """
        )
        for line in r["stdout"].splitlines():
            self.assertTrue(line.endswith(("EROFS", "EACCES", "EPERM")), line)

    def test_every_host_bound_mount_is_read_only(self):
        r = sandbox(
            """
            for line in open("/proc/self/mountinfo"):
                f = line.split()
                print(f[4], f[5])
            """
        )
        mounts = dict(line.split(" ", 1) for line in r["stdout"].splitlines())
        for mp, opts in mounts.items():
            ro = "ro" in opts.split(",")
            if mp == "/work":
                self.assertFalse(ro, mp)
            elif mp.startswith(("/usr", "/bin", "/lib", "/sbin", "/etc", "/dev")) and mp not in (
                "/dev/null", "/dev/zero", "/dev/full", "/dev/urandom", "/dev/random"
            ):
                self.assertTrue(ro, f"{mp} is writable: {opts}")
        self.assertTrue("ro" in mounts["/"].split(","), "root must be read-only")

    def test_work_directory_is_writable_but_not_executable(self):
        r = sandbox(
            """
            import os, stat, subprocess
            open("tool", "w").write("#!/bin/sh\\necho pwned\\n")
            os.chmod("tool", 0o755)
            try:
                subprocess.run(["/work/tool"], check=True)
                print("executed")
            except OSError as e:
                print("denied", e.errno)
            """
        )
        self.assertEqual(r["stdout"].split()[0], "denied", r)

    def test_environment_is_locked_down(self):
        r = sandbox("import os\nprint(sorted(os.environ.items()))")
        env = dict(eval(r["stdout"]))  # noqa: S307
        self.assertEqual(
            env,
            {"PATH": "/usr/bin:/bin", "HOME": "/work", "PYTHONHASHSEED": "0", "LC_ALL": "C.UTF-8", "TZ": "UTC"},
        )

    def test_no_work_directory_escape_through_proc(self):
        r = sandbox(
            """
            import os
            print(os.path.realpath("/proc/self/root"), sorted(os.listdir("/proc/1/root"))[:3] == sorted(os.listdir("/"))[:3])
            """
        )
        self.assertEqual(r["stdout"], "/ True\n")


class Syscalls(unittest.TestCase):
    def violation(self, code, nr=None):
        r = sandbox(code)
        self.assertEqual(r["termination_reason"], "SECURITY_VIOLATION", r)
        self.assertIsNotNone(r["violation"])
        if nr is not None and X86_64:
            self.assertEqual(r["violation"]["syscall_nr"], nr)
        return r

    def test_network_socket_is_denied_with_its_number(self):
        r = self.violation("import socket\nsocket.socket(socket.AF_INET, socket.SOCK_STREAM)", nr=41)
        self.assertEqual(r["stdout"], "")

    def test_reverse_shell_attempt(self):
        self.violation(
            """
            import socket, subprocess, os
            s = socket.create_connection(("127.0.0.1", 4444))
            """,
            nr=41,
        )

    @unittest.skipUnless(X86_64, "syscall numbers below are x86-64")
    def test_dangerous_syscalls(self):
        cases = {
            "ptrace": 101,
            "mount": 165,
            "reboot": 169,
            "unshare": 272,
            "setns": 308,
            "pivot_root": 155,
            "init_module": 175,
            "kexec_load": 246,
            "bpf": 321,
            "userfaultfd": 323,
            "perf_event_open": 298,
            "setuid": 105,
        }
        for name, nr in cases.items():
            with self.subTest(syscall=name):
                self.violation(f"import ctypes\nctypes.CDLL(None, use_errno=True).syscall({nr}, 0, 0, 0, 0, 0)", nr=nr)

    def test_namespace_creation_by_clone_is_denied(self):
        r = sandbox(
            """
            import os
            CLONE_NEWUSER = 0x10000000
            try:
                os.unshare(CLONE_NEWUSER)
            except AttributeError:
                import ctypes
                ctypes.CDLL(None).unshare(CLONE_NEWUSER)
            """
        )
        self.assertEqual(r["termination_reason"], "SECURITY_VIOLATION", r)

    def test_soft_denied_clone3_does_not_break_threads(self):
        r = sandbox("import threading\nt = threading.Thread(target=print, args=('ok',)); t.start(); t.join()")
        self.assertEqual(r["stdout"], "ok\n")
        self.assertIsNone(r["violation"])


class ResultChannel(unittest.TestCase):
    def test_forged_json_on_stdout_changes_nothing(self):
        forged = json.dumps({"schema_version": 1, "termination_reason": "EXITED", "exit_code": 0, "stdout": "PASSED"})
        r = sandbox(f"import sys\nprint({forged!r})\nsys.exit(7)")
        self.assertEqual(r["exit_code"], 7, "the exit status comes from wait4(), not from what the child prints")
        self.assertEqual(r["termination_reason"], "EXITED")
        self.assertIn("PASSED", r["stdout"])

    def test_forged_json_on_stderr_and_closed_descriptors(self):
        r = sandbox(
            """
            import os, sys
            sys.stderr.write('{"termination_reason": "EXITED", "exit_code": 0}')
            os.close(1); os.close(2)
            os._exit(9)
            """
        )
        self.assertEqual(r["exit_code"], 9)

    def test_no_leak_of_the_secret_input_through_proc(self):
        secret = "TOP-SECRET-TESTCASE-INPUT-91827"
        with tempfile.TemporaryDirectory(dir=RUN_DIR) as tmp:
            code = Path(tmp) / "main.py"
            code.write_text("import time\nimport sys\nsys.stdin.read()\ntime.sleep(1.5)")
            p = subprocess.Popen(base_args(code), stdin=subprocess.PIPE, stdout=subprocess.PIPE)
            p.stdin.write(secret.encode())
            p.stdin.close()
            time.sleep(0.7)
            leaked = []
            for pid in filter(str.isdigit, os.listdir("/proc")):
                try:
                    if secret.encode() in Path(f"/proc/{pid}/cmdline").read_bytes():
                        leaked.append(pid)
                except OSError:
                    pass
            p.communicate()
        self.assertEqual(leaked, [], "test input must travel on stdin, never in argv")


class Determinism(unittest.TestCase):
    def test_set_ordering_is_stable_across_runs(self):
        code = 'print(set("hello world"))\nprint({1, 2, "a", "b"})\nprint(hash("abc"))\nprint(list({"x": 1, "y": 2}))'
        outputs = {sandbox(code)["stdout"] for _ in range(20)}
        self.assertEqual(len(outputs), 1, outputs)

    def test_time_zone_and_locale(self):
        r = sandbox("import time, locale\nprint(time.tzname, locale.getpreferredencoding())")
        self.assertEqual(r["stdout"], "('UTC', 'UTC') UTF-8\n")

    def test_isolated_mode_would_defeat_the_fixed_hash_seed(self):
        # Regression guard for the command line: -I implies -E and drops PYTHONHASHSEED.
        code = 'print(hash("abc"))'
        with tempfile.TemporaryDirectory(dir=RUN_DIR) as tmp:
            path = Path(tmp) / "main.py"
            path.write_text(code)
            args = base_args(path)
            args[args.index("-s")] = "-I"
            args.remove("-P")
            seen = {json.loads(subprocess.run(args, input=b"", capture_output=True).stdout)["stdout"]
                    for _ in range(6)}
        self.assertGreater(len(seen), 1, "-I unexpectedly honoured PYTHONHASHSEED")


class Supervision(unittest.TestCase):
    def test_supervisor_sigkill_leaves_nothing_running_and_sweep_cleans_up(self):
        with tempfile.TemporaryDirectory(dir=RUN_DIR) as tmp:
            code = Path(tmp) / "main.py"
            code.write_text("import time\ntime.sleep(3600)")
            p = subprocess.Popen(base_args(code, timeout_s=3000, cpu_time_ms=3000000), stdin=subprocess.DEVNULL,
                                 stdout=subprocess.PIPE)
            deadline = time.monotonic() + 10
            cg = None
            while time.monotonic() < deadline and cg is None:
                found = [n for n in sbx_entries(CGROUP_ROOT) if n.startswith(f"sbx-{p.pid}-")]
                if found and Path(CGROUP_ROOT, found[0], "cgroup.procs").read_text().strip():
                    cg = Path(CGROUP_ROOT, found[0])
                time.sleep(0.05)
            self.assertIsNotNone(cg, "sandbox did not start")
            p.kill()
            p.wait()
            p.stdout.close()
            # PR_SET_PDEATHSIG: the sandbox dies with its supervisor.
            deadline = time.monotonic() + 5
            while time.monotonic() < deadline and cg.joinpath("cgroup.procs").read_text().strip():
                time.sleep(0.05)
            self.assertEqual(cg.joinpath("cgroup.procs").read_text().strip(), "", "sandbox outlived its supervisor")
            self.assertTrue(sbx_entries(CGROUP_ROOT), "the crashed run should have left a cgroup to sweep")
            swept = subprocess.run([BIN, "--sweep", "--cgroup-root", CGROUP_ROOT, "--run-dir", RUN_DIR],
                                   capture_output=True, text=True)
            self.assertEqual(swept.returncode, 0, swept.stderr)
        self.assertEqual(sbx_entries(CGROUP_ROOT), [])
        self.assertEqual(sbx_entries(RUN_DIR), [])

    def test_sigterm_ends_the_run_and_cleans_up(self):
        with tempfile.TemporaryDirectory(dir=RUN_DIR) as tmp:
            code = Path(tmp) / "main.py"
            code.write_text("import time\ntime.sleep(3600)")
            p = subprocess.Popen(base_args(code, timeout_s=3000, cpu_time_ms=3000000), stdin=subprocess.DEVNULL,
                                 stdout=subprocess.PIPE)
            time.sleep(1.0)
            p.send_signal(signal.SIGTERM)
            out, _ = p.communicate(timeout=10)
        rep = json.loads(out)
        self.assertEqual(rep["termination_reason"], "INTERNAL_ERROR")
        self.assertEqual(p.returncode, 70)
        self.assertIn("signal", rep["error"])


class Usage(unittest.TestCase):
    def run_bad(self, *extra, replace=None):
        args = [BIN, "--cgroup-root", CGROUP_ROOT, "--run-dir", RUN_DIR, *limit_args(), *extra]
        if replace:
            args = replace
        return subprocess.run(args, input=b"", capture_output=True)

    def test_missing_command(self):
        self.assertEqual(self.run_bad().returncode, 2)

    def test_relative_command_is_refused(self):
        self.assertEqual(self.run_bad("--", "python3").returncode, 2)

    def test_missing_limit(self):
        proc = subprocess.run([BIN, "--cgroup-root", CGROUP_ROOT, "--", "/usr/bin/python3"], capture_output=True)
        self.assertEqual(proc.returncode, 2)

    def test_tmpfs_must_exceed_twice_the_output_cap(self):
        proc = self.run_bad("--", "/usr/bin/python3", replace=[
            BIN, "--cgroup-root", CGROUP_ROOT, *limit_args(max_output_bytes=20 << 20, tmpfs_size_mb=32, memory_mb=128),
            "--", "/usr/bin/python3"])
        self.assertEqual(proc.returncode, 2)
        self.assertIn(b"invariant", proc.stderr)

    def test_tmpfs_needs_headroom_below_the_memory_limit(self):
        proc = subprocess.run([BIN, "--cgroup-root", CGROUP_ROOT, *limit_args(tmpfs_size_mb=32, memory_mb=40),
                               "--", "/usr/bin/python3"], capture_output=True)
        self.assertEqual(proc.returncode, 2)
        self.assertIn(b"invariant", proc.stderr)

    def test_headroom_boundary_is_accepted(self):
        r = sandbox('print("ok")', tmpfs_size_mb=32, memory_mb=48)
        self.assertEqual(r["stdout"], "ok\n")

    def test_copy_destination_cannot_escape_work(self):
        for dest in ("../x", "/abs", "a//b", "a/./b"):
            with self.subTest(dest=dest):
                proc = subprocess.run([BIN, "--cgroup-root", CGROUP_ROOT, *limit_args(), "--copy", f"/etc/hostname:{dest}",
                                       "--", "/usr/bin/python3"], capture_output=True)
                self.assertEqual(proc.returncode, 2)

    def test_internal_error_is_exit_70_with_a_report(self):
        proc = subprocess.run([BIN, "--cgroup-root", "/nonexistent-cgroup", "--run-dir", RUN_DIR, *limit_args(),
                               "--", "/usr/bin/python3"], capture_output=True)
        self.assertEqual(proc.returncode, 70)
        rep = json.loads(proc.stdout)
        self.assertEqual(rep["termination_reason"], "INTERNAL_ERROR")
        self.assertTrue(rep["error"])

    def test_unknown_executable_is_internal_error(self):
        with tempfile.TemporaryDirectory(dir=RUN_DIR) as tmp:
            code = Path(tmp) / "main.py"
            code.write_text("")
            args = base_args(code)
            args[args.index("/usr/bin/python3")] = "/usr/bin/does-not-exist"
            proc = subprocess.run(args, input=b"", capture_output=True)
        self.assertEqual(proc.returncode, 70)
        self.assertIn("execve", json.loads(proc.stdout)["error"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
