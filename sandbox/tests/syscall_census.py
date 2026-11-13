#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Which system calls do typical student programs need, and does the seccomp allowlist cover them?

The allowlist in src/filter.c says it is "what CPython and its standard library need", and names
`strace -f -c` as the way to check. This script does that over a corpus of small programs of the
kinds students write, in two modes.

  syscall_census.py strace [--python PATH] [--strace PATH] [--json FILE]
      Runs every program of the corpus under `strace -f`, outside the sandbox, with the
      interpreter flags and environment the Grading Harness uses, and counts the calls in the
      full trace (the `-c` summary leaves out calls that never return, such as exit_group and a
      successful execve, which are on the allowlist). Prints the calls the corpus
      used that the filter does not allow, the calls the filter allows that no program used,
      and checks the arguments of the calls the filter allows only with certain arguments
      (ioctl, prctl, clone, socketpair) against the ones the programs passed.

  tests/run.sh /abs/path/to/sandbox-exec --exec python3 tests/syscall_census.py sandbox
      Runs every program of the corpus in the real sandbox with generous limits and prints how
      each one ended. A program that ends in SECURITY_VIOLATION used a call the filter refuses.

Needs Linux and strace; the sandbox mode also needs what tests/run.sh needs. Python standard
library only. What it shows depends on the interpreter, the C library and the kernel it runs on, so
state them with the result. A call that no program in the corpus used is not proof that no student
program needs it.
"""
import argparse
import collections
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import textwrap
from pathlib import Path

HERE = Path(__file__).resolve().parent
FILTER_C = HERE.parent / "src" / "filter.c"

# name: (source, stdin). Each program is small, deterministic and finishes in well under a second.
CORPUS = {
    "hello": ('print("hello")', ""),
    "stdin_sum": ("import sys\nprint(sum(map(int, sys.stdin.read().split())))", "1 2 3\n"),
    "input_loop": ("n = int(input())\nfor _ in range(n):\n    print(int(input()) * 2)", "3\n1\n2\n3\n"),
    "eof": ("try:\n    input()\nexcept EOFError:\n    print('eof')", ""),
    "containers": """
        import collections
        c = collections.Counter("abracadabra"); d = collections.defaultdict(list); q = collections.deque(range(5))
        d["a"].append(1); s = {3, 1, 2}
        print(sorted(c.items()), dict(d), list(q), s, sorted(s, reverse=True), {1: 'a', 2: 'b'})
        """,
    "recursion": """
        import sys
        sys.setrecursionlimit(5000)
        def f(n): return 1 if n < 2 else f(n - 1) + f(n - 2)
        def depth(n): return 0 if n == 0 else 1 + depth(n - 1)
        print(f(18), depth(3000))
        """,
    "numbers": """
        import math, decimal, fractions, statistics, cmath
        print(math.sqrt(2), math.gcd(12, 18), decimal.Decimal("1.1") + decimal.Decimal("2.2"),
              fractions.Fraction(1, 3) + 1, statistics.mean([1, 2, 3, 4]), cmath.sqrt(-1), 2 ** 200)
        """,
    "text": """
        import re, string, textwrap, unicodedata, locale
        print(re.findall(r"\\d+", "a1b22c333"), string.ascii_letters[:5], textwrap.fill("x " * 50, 30)[:20],
              unicodedata.name("a"), f"{3.14159:.2f}", "abc".upper(), locale.getpreferredencoding())
        """,
    "data_formats": """
        import json, csv, io, base64, struct, array, hashlib, zlib
        buf = io.StringIO(); csv.writer(buf).writerow([1, "a,b", 3])
        print(json.loads(json.dumps({"a": [1, 2]})), buf.getvalue().strip(), base64.b64encode(b"hi"),
              struct.pack("<I", 7), array.array("i", [1, 2]), hashlib.sha256(b"x").hexdigest()[:8],
              hashlib.md5(b"x").hexdigest()[:8], len(zlib.compress(b"a" * 1000)))
        """,
    "functional": """
        import itertools, functools, heapq, bisect, operator, dataclasses, typing, enum
        @dataclasses.dataclass
        class P: x: int; y: int = 0
        class C(enum.Enum): A = 1
        print(list(itertools.permutations(range(3)))[:2], functools.reduce(operator.add, range(5)),
              heapq.nsmallest(2, [5, 1, 3]), bisect.bisect([1, 3], 2), P(1), C.A,
              functools.lru_cache(maxsize=None)(lambda n: n)(3))
        """,
    "time_basic": """
        import time, datetime
        t = time.time(); time.sleep(0.02)
        print(time.perf_counter() > 0, time.monotonic() > 0, time.process_time() >= 0,
              datetime.datetime.now(datetime.timezone.utc).year, time.strftime("%Y", time.localtime(t)))
        """,
    "random": """
        import random, os, secrets
        random.seed(1); l = list(range(10)); random.shuffle(l)
        print(random.random(), l, len(os.urandom(8)), len(secrets.token_hex(8)))
        """,
    "files_work": """
        import os, shutil, pathlib
        with open("a.txt", "w") as f: f.write("x\\n" * 100)
        with open("a.txt", "a") as f: f.write("y\\n")
        os.makedirs("d/e", exist_ok=True); shutil.copy("a.txt", "d/b.txt"); os.rename("d/b.txt", "d/c.txt")
        print(len(open("a.txt").read()), os.listdir("d"), os.stat("a.txt").st_size, sorted(pathlib.Path(".").glob("**/*.txt")))
        shutil.rmtree("d"); os.remove("a.txt")
        """,
    "tempfile": """
        import tempfile
        with tempfile.NamedTemporaryFile("w+") as f:
            f.write("x"); f.flush(); f.seek(0); print(f.read())
        with tempfile.TemporaryDirectory() as d: print(bool(d))
        """,
    "threads": """
        import threading, queue, concurrent.futures
        q = queue.Queue(); lock = threading.Lock(); n = [0]
        def w():
            for i in range(100):
                with lock: n[0] += 1
            q.put(1)
        ts = [threading.Thread(target=w) for _ in range(4)]
        [t.start() for t in ts]; [t.join() for t in ts]
        with concurrent.futures.ThreadPoolExecutor(4) as ex: r = list(ex.map(lambda x: x * x, range(8)))
        print(n[0], q.qsize(), r)
        """,
    "processes_fork": """
        import multiprocessing as mp
        def sq(x): return x * x
        if __name__ == "__main__":
            with mp.get_context("fork").Pool(2) as p: print(p.map(sq, range(6)))
        """,
    "processes_default": """
        import multiprocessing as mp
        def sq(x): return x * x
        if __name__ == "__main__":
            with mp.Pool(2) as p: print(p.map(sq, range(6)))
        """,
    "os_fork": """
        import os
        pid = os.fork()
        if pid == 0: os._exit(3)
        print(os.waitpid(pid, 0)[1] >> 8)
        """,
    "subprocess_python": """
        import subprocess, sys
        print(subprocess.run([sys.executable, "-c", "print(1)"], capture_output=True, text=True).stdout.strip())
        """,
    "asyncio": """
        import asyncio
        async def w(i):
            await asyncio.sleep(0.01); return i
        async def main(): return await asyncio.gather(*(w(i) for i in range(5)))
        print(asyncio.run(main()))
        """,
    "socketpair_selectors": """
        import socket, selectors
        a, b = socket.socketpair(); sel = selectors.DefaultSelector(); sel.register(b, selectors.EVENT_READ)
        a.send(b"x"); print(len(sel.select(1)), b.recv(1))
        """,
    "uncaught_exception": "print('before')\n1 / 0",
    "exit_code": "import sys\nprint('x')\nsys.exit(3)",
    "assert_fail": "assert 1 == 2, 'no'",
    "memory_big": """
        import gc
        x = [0] * 3_000_000; b = bytearray(30_000_000); b[0] = 1
        del x, b; gc.collect(); print("ok")
        """,
    "output_big": "for i in range(100000):\n    print(i)",
    "imports_wide": """
        import argparse, logging, unittest, pickle, sqlite3, ctypes, ssl, http.client, urllib.request, socket
        import xml.etree.ElementTree, email, html.parser, zipfile, tarfile, bz2, lzma, uuid, platform, shelve, glob
        print(platform.system(), uuid.uuid4().version, sqlite3.sqlite_version[:1])
        """,
    "os_info": """
        import os, sys, resource, signal, platform
        def h(*a): pass
        signal.signal(signal.SIGALRM, h); signal.setitimer(signal.ITIMER_REAL, 0.01); signal.pause()
        print(os.getpid() > 0, os.getppid() >= 0, os.getuid() >= 0, os.getcwd(), os.cpu_count(), os.uname().sysname,
              sys.stdout.isatty(), resource.getrlimit(resource.RLIMIT_NOFILE)[0] > 0, resource.getrusage(0).ru_maxrss > 0,
              platform.machine())
        try: print(os.get_terminal_size())
        except OSError: print("no tty")
        """,
    "ctypes_mmap": """
        import ctypes, mmap
        m = mmap.mmap(-1, 4096); m[0:2] = b"ab"
        print(m[0:2], ctypes.CDLL(None).getpid() > 0)
        """,
    "unittest_main": """
        import unittest
        class T(unittest.TestCase):
            def test_a(self): self.assertEqual(1 + 1, 2)
        unittest.main(argv=["x"])
        """,
    "cpu_only": "print(sum(i * i for i in range(300000)))",
}

# What the filter allows only with certain arguments (arg_rules in src/filter.c), as strace names them.
IOCTL_OK = {"TCGETS", "TCGETS2", "TIOCGWINSZ", "FIONREAD", "FIONBIO", "FIOCLEX", "FIONCLEX"}
PRCTL_OK = {"PR_SET_NAME", "PR_GET_NAME", "PR_GET_DUMPABLE", "PR_SET_DUMPABLE", "PR_SET_TIMERSLACK",
            "PR_GET_TIMERSLACK", "PR_GET_TID_ADDRESS", "PR_GET_NO_NEW_PRIVS"}
CLONE_FORBIDDEN = ("CLONE_NEWNS", "CLONE_NEWUSER", "CLONE_NEWPID", "CLONE_NEWNET", "CLONE_NEWIPC", "CLONE_NEWUTS",
                   "CLONE_NEWCGROUP", "CLONE_NEWTIME", "CLONE_UNTRACED")


def entry(name):
    v = CORPUS[name]
    return v if isinstance(v, tuple) else (v, "")


def source_of(name):
    return textwrap.dedent(entry(name)[0]).lstrip("\n")


def stdin_of(name):
    return entry(name)[1].encode()


def allowlist():
    """The names in filter.c: allowed outright, allowed with arguments, answered ENOSYS."""
    text = FILTER_C.read_text()

    def block(start):
        i = text.index(start)
        return text[i:text.index("};", i)]

    quoted = lambda s: re.findall(r'"([a-z0-9_]+)"', s)
    allowed = set(quoted(block("static const char *const allowed[]")))
    enosys = set(quoted(block("static const char *const enosys[]")))
    rules = {m.group(1) for m in re.finditer(r'\{ "([a-z0-9_]+)", \d+, SCMP_CMP', text)}
    return allowed, rules, enosys


def clean_env(home):
    return {"PATH": "/usr/bin:/bin", "HOME": home, "PYTHONHASHSEED": "0", "LC_ALL": "C.UTF-8", "TZ": "UTC"}


CALL = re.compile(r"^(?:\d+\s+)?(?:\[pid\s+\d+\]\s+)?([a-z][a-z0-9_]*)\(")


def parse_trace(path):
    """Calls by name, and the lines of the calls whose arguments the filter checks.

    The full trace is read rather than the `-c` summary: the summary leaves out calls that never
    return, which includes exit_group and a successful execve, and both are on the allowlist.
    """
    counts = collections.Counter()
    checked = []
    for line in Path(path).read_text(errors="replace").splitlines():
        m = CALL.match(line)
        if not m:
            continue
        counts[m.group(1)] += 1
        if m.group(1) in ("ioctl", "prctl", "clone", "clone3", "socketpair"):
            checked.append(CALL.sub(lambda x: x.group(1) + "(", line))
    return counts, checked


def arg_problem(line):
    """The call name if this strace line is a call the filter would refuse for its arguments, else None.

    Mirrors arg_rules in src/filter.c. A request or option that strace prints as a number, because
    it has no name for it, is reported: the filter compares numbers, so it cannot be told apart here.
    """
    if line.startswith("ioctl("):
        m = re.match(r"ioctl\(\d+, ([A-Za-z0-9_|]+)", line)
        return "ioctl" if not m or m.group(1) not in IOCTL_OK else None
    if line.startswith("prctl("):
        m = re.match(r"prctl\(([A-Za-z0-9_|x]+)", line)
        return "prctl" if not m or m.group(1) not in PRCTL_OK else None
    if line.startswith(("clone(", "clone3(")):
        return "clone" if any(f in line for f in CLONE_FORBIDDEN) else None
    if line.startswith("socketpair("):
        return "socketpair" if not line.startswith("socketpair(AF_UNIX") else None
    return None


def census(args):
    strace = args.strace or shutil.which("strace")
    if not strace:
        sys.exit("strace not found; pass --strace PATH")
    py = args.python
    allowed, with_args, enosys = allowlist()
    ok = allowed | with_args | enosys
    used = collections.defaultdict(dict)  # call -> {program: count}
    arg_findings = collections.defaultdict(set)
    status = {}
    for name in CORPUS:
        with tempfile.TemporaryDirectory() as d:
            Path(d, "main.py").write_text(source_of(name))
            cmd = [py, "-s", "-P", "-B", "main.py"]
            env = clean_env(d)
            hide = []
            if "LD_LIBRARY_PATH" in os.environ:
                # a strace unpacked without root needs it to start; the program must not see it
                env["LD_LIBRARY_PATH"] = os.environ["LD_LIBRARY_PATH"]
                hide = ["-E", "LD_LIBRARY_PATH"]
            trace = Path(d, "trace.txt")
            p = subprocess.run([strace, "-f", *hide, "-o", str(trace), *cmd], cwd=d, env=env, input=stdin_of(name),
                               capture_output=True, timeout=120)
            status[name] = p.returncode
            counts, checked = parse_trace(trace)
            for call, n in counts.items():
                used[call][name] = n
            for line in checked:
                call = arg_problem(line)
                if call:
                    arg_findings[call].add((name, line[:90]))
    not_allowed = {c: p for c, p in used.items() if c not in ok}
    unused = sorted(allowed - set(used))
    print(f"programs: {len(CORPUS)}; interpreter: {subprocess.run([py, '--version'], capture_output=True, text=True).stdout.strip()}; "
          f"{strace} {subprocess.run([strace, '--version'], capture_output=True, text=True).stdout.splitlines()[0]}; "
          f"kernel {os.uname().release}")
    print("programs that did not exit 0 (expected: uncaught_exception 1, exit_code 3, assert_fail 1):",
          {n: s for n, s in status.items() if s != 0})
    print(f"\ndistinct calls used by the corpus: {len(used)}")
    print(f"used and allowed outright:            {len([c for c in used if c in allowed])}")
    print(f"used, allowed with arguments:         {sorted(c for c in used if c in with_args)}")
    print(f"used, answered ENOSYS by design:      {sorted(c for c in used if c in enosys)}")
    print(f"USED BUT NOT ON THE LIST ({len(not_allowed)}):")
    for c, p in sorted(not_allowed.items()):
        print(f"  {c}: {sorted(p)}")
    print(f"\nargument checks (a line is a call the filter would refuse):")
    for k in ("ioctl", "prctl", "clone", "socketpair"):
        for prog, line in sorted(arg_findings[k]):
            print(f"  {k} in {prog}: {line}")
    if not any(arg_findings.values()):
        print("  none: every ioctl, prctl, clone and socketpair call used an argument the filter allows")
    print(f"\nALLOWED BUT NOT USED by any program ({len(unused)} of {len(allowed)}):")
    print(textwrap.fill(", ".join(unused), 100, initial_indent="  ", subsequent_indent="  "))
    if args.json:
        Path(args.json).write_text(json.dumps({
            "used": {c: p for c, p in sorted(used.items())}, "not_allowed": not_allowed, "unused": unused,
            "exit_status": status, "arg_findings": {k: sorted(v) for k, v in arg_findings.items()}}, indent=1))


def syscall_names():
    for h in ("/usr/include/x86_64-linux-gnu/asm/unistd_64.h", "/usr/include/asm/unistd_64.h"):
        if os.path.exists(h):
            return {int(m.group(2)): m.group(1) for m in re.finditer(r"#define __NR_(\w+) (\d+)", open(h).read())}
    return {}


def sandbox_run(args):
    binary, cg = os.environ["SBX_BIN"], os.environ["SBX_CGROUP_ROOT"]
    names = syscall_names()
    run_dir = tempfile.mkdtemp(prefix="census-")
    bad = 0
    for name in CORPUS:
        with tempfile.TemporaryDirectory(dir=run_dir) as d:
            Path(d, "main.py").write_text(source_of(name))
            cmd = [binary, "--cgroup-root", cg, "--run-dir", run_dir, "--cpu-time-ms", "5000", "--timeout-s", "20",
                   "--memory-mb", "256", "--max-processes", "16", "--max-output-bytes", "1048576", "--tmpfs-size-mb", "32",
                   "--copy", f"{d}/main.py:main.py", "--", "/usr/bin/python3", "-s", "-P", "-B", "main.py"]
            p = subprocess.run(cmd, input=stdin_of(name), capture_output=True, timeout=120)
            try:
                rep = json.loads(p.stdout)
            except ValueError:
                print(f"{name:22s} NO REPORT: {p.stderr.decode(errors='replace')[:100]}")
                bad += 1
                continue
            v = (rep.get("violation") or {}).get("syscall_nr")
            tail = f" violation {v} ({names.get(v, '?')})" if v is not None else ""
            err = rep.get("stderr", "").strip().splitlines()
            note = f"  | {err[-1][:90]}" if rep["exit_code"] not in (0, None) and err else ""
            print(f"{name:22s} {rep['termination_reason']:20s} exit {rep['exit_code']}{tail}{note}")
            if rep["termination_reason"] == "SECURITY_VIOLATION":
                bad += 1
    shutil.rmtree(run_dir, ignore_errors=True)
    print(f"\n{bad} of {len(CORPUS)} programs were stopped by the filter or gave no report")


if __name__ == "__main__":
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="mode", required=True)
    a = sub.add_parser("strace")
    a.add_argument("--python", default="/usr/bin/python3")
    a.add_argument("--strace")
    a.add_argument("--json")
    a.set_defaults(fn=census)
    b = sub.add_parser("sandbox")
    b.set_defaults(fn=sandbox_run)
    ns = ap.parse_args()
    ns.fn(ns)
