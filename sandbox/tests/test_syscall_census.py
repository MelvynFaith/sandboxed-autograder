# SPDX-License-Identifier: MIT
"""Tests of the parts of syscall_census.py that decide its answer: reading a trace, reading the
allowlist out of src/filter.c, and judging the arguments of the calls the filter checks.
No strace and no sandbox needed:  python3 -m unittest tests/test_syscall_census.py  (from sandbox/)."""
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import syscall_census as sc  # noqa: E402


class AllowlistTest(unittest.TestCase):
    def setUp(self):
        self.allowed, self.with_args, self.enosys = sc.allowlist()

    def test_the_lists_are_read_from_filter_c(self):
        for name in ("read", "write", "exit_group", "execve", "futex", "newfstatat", "getrandom"):
            self.assertIn(name, self.allowed)
        self.assertGreater(len(self.allowed), 150)

    def test_calls_that_are_refused_are_not_on_it(self):
        for name in ("socket", "connect", "bind", "listen", "accept4", "ptrace", "mount", "unshare", "io_uring_setup"):
            self.assertNotIn(name, self.allowed | self.with_args | self.enosys)

    def test_calls_allowed_only_with_arguments(self):
        self.assertEqual(self.with_args, {"clone", "socketpair", "ioctl", "prctl"})
        self.assertFalse(self.with_args & self.allowed)

    def test_calls_answered_with_enosys(self):
        self.assertEqual(self.enosys, {"clone3", "openat2"})


class TraceTest(unittest.TestCase):
    def parse(self, text):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d, "t")
            p.write_text(text)
            return sc.parse_trace(p)

    def test_counts_plain_pid_prefixed_and_unfinished_lines_once(self):
        counts, _ = self.parse(
            "100   read(0, \"\", 4096) = 0\n"
            "[pid   101] write(1, \"x\", 1 <unfinished ...>\n"
            "[pid   101] <... write resumed>) = 1\n"
            "100   exit_group(0)                  = ?\n"
            "+++ exited with 0 +++\n"
            "--- SIGCHLD {si_signo=SIGCHLD} ---\n")
        self.assertEqual(dict(counts), {"read": 1, "write": 1, "exit_group": 1})

    def test_keeps_the_lines_whose_arguments_matter_without_the_pid(self):
        _, checked = self.parse("7  ioctl(1, TCGETS, 0x7ffd) = -1\n7  read(0, \"\", 1) = 0\n[pid 8] prctl(PR_SET_NAME, \"x\") = 0\n")
        self.assertEqual(checked, ["ioctl(1, TCGETS, 0x7ffd) = -1", "prctl(PR_SET_NAME, \"x\") = 0"])


class ArgumentTest(unittest.TestCase):
    def test_requests_the_filter_allows(self):
        for line in ("ioctl(1, TCGETS, {c_iflag=0}) = 0", "ioctl(3, FIONBIO, [1]) = 0", "ioctl(0, TIOCGWINSZ, ...) = -1",
                     "ioctl(5, FIOCLEX) = 0", "prctl(PR_SET_NAME, \"python3\") = 0", "prctl(PR_GET_NO_NEW_PRIVS, 0) = 1",
                     "clone(child_stack=NULL, flags=CLONE_CHILD_CLEARTID|CLONE_CHILD_SETTID|SIGCHLD) = 9",
                     "socketpair(AF_UNIX, SOCK_STREAM, 0, [3, 4]) = 0", "read(0, \"\", 1) = 0"):
            self.assertIsNone(sc.arg_problem(line), line)

    def test_requests_the_filter_would_refuse(self):
        cases = {
            "ioctl(0, TIOCSTI, \"x\") = 0": "ioctl",
            "ioctl(3, 0xdeadbeef, 0) = -1": "ioctl",
            "prctl(PR_SET_VMA, PR_SET_VMA_ANON_NAME, 0x7f, 4096, \"x\") = 0": "prctl",
            "prctl(0x53564d41, 0, 0) = -1": "prctl",
            "clone(child_stack=NULL, flags=CLONE_NEWUSER|SIGCHLD) = -1": "clone",
            "clone(child_stack=NULL, flags=CLONE_VM|CLONE_UNTRACED|SIGCHLD) = 3": "clone",
            "clone3({flags=CLONE_NEWNS|CLONE_VM, exit_signal=SIGCHLD}, 88) = -1": "clone",
            "socketpair(AF_INET, SOCK_STREAM, 0, [3, 4]) = 0": "socketpair",
        }
        for line, call in cases.items():
            self.assertEqual(sc.arg_problem(line), call, line)


class CorpusTest(unittest.TestCase):
    def test_every_program_compiles_and_has_a_name(self):
        self.assertGreaterEqual(len(sc.CORPUS), 25)
        for name in sc.CORPUS:
            compile(sc.source_of(name), name, "exec")
            self.assertIsInstance(sc.stdin_of(name), bytes)

    def test_a_program_given_as_a_bare_string_is_not_cut_to_its_first_letter(self):
        self.assertTrue(sc.source_of("exit_code").startswith("import sys"))
        self.assertEqual(sc.stdin_of("exit_code"), b"")
        self.assertEqual(sc.stdin_of("stdin_sum"), b"1 2 3\n")


if __name__ == "__main__":
    unittest.main()
