# Which system calls do typical programs use? (13 Nov 2026)

The seccomp allowlist in `sandbox/src/filter.c` is described as "what CPython and its standard library
need", and the audit of 12 Nov ([adversarial report](adversarial-report.md), "Audit of the syscall
allowlist") ended with a gap: it did not measure which of the allowlisted calls CPython uses for typical
programs. This note measures that, with `sandbox/tests/syscall_census.py`. It changes nothing in the
filter; whether to narrow it is a separate decision.

## Method

- A corpus of 31 small programs of the kinds students write: reading standard input, containers and
  recursion, numbers and text, JSON and CSV, `itertools` and `functools`, time and random, files in the
  working directory, `tempfile`, threads and thread pools, `os.fork` and `subprocess`, `multiprocessing`
  with the `fork` start method and with the default one, `asyncio`, `socketpair` with `selectors`,
  uncaught exceptions and exit codes, 30 MB of memory, 100 000 lines of output, a wide `import` of the
  standard library, `os` and `resource` queries and signals, `ctypes` and `mmap`, `unittest`.
- Each program runs outside the sandbox under `strace -f`, as `python3 -s -P -B main.py` with the
  environment the Grading Harness gives (`PYTHONHASHSEED=0`, `LC_ALL=C.UTF-8`, `TZ=UTC`, nothing else).
  The full trace is counted, not the `-c` summary, which leaves out calls that never return: `exit_group`
  and a successful `execve`, both on the allowlist.
- The names are compared with the lists read out of `filter.c`: allowed, allowed only with an argument
  (`clone`, `socketpair`, `ioctl`, `prctl`), and answered with `ENOSYS` (`clone3`, `openat2`). For the
  four calls with a rule, each call of the trace is checked against the rule.
- The same programs also run in the real sandbox, to see whether any is stopped by the filter.

Environment: Ubuntu 26.04 (WSL2), kernel 6.18.33.2-microsoft-standard-WSL2, glibc 2.43, Python 3.14.4,
strace 6.19, x86-64. **This is not the campus server**, which runs Ubuntu 24.04, kernel 6.8.0-139 and
Python 3.12.3. The calls a program makes depend on the interpreter, the C library and the kernel, so the
result must be repeated there before it is relied on (commands at the end). It was, on 14 Nov: see "Campus server
run" below.

## Results

Calls used by the corpus: 83 distinct.

| Class | Number |
|---|---:|
| On the allowlist outright | 73 |
| Allowed with an argument rule: `clone`, `ioctl`, `prctl`, `socketpair` | 4 |
| Answered with `ENOSYS` by design: `clone3` (4 programs; glibc retries with `clone`) | 1 |
| **Not on the list**: `socket`, `bind`, `listen`, `connect`, `accept4` | 5 |

- **The argument rules hold for everything the corpus did.** Every `ioctl` (196 calls) asked for a request
  the filter allows, the 13 `prctl` calls (threads, `multiprocessing`) used allowed options, `clone` never
  carried a namespace flag or `CLONE_UNTRACED`, and `socketpair` always asked for `AF_UNIX`.
- **The five calls that are not on the list come from one program.** `multiprocessing.Pool()` with the
  default start method opens an `AF_UNIX` listening socket for the fork server, which is the default on
  Linux from Python 3.14 (Python 3.12 defaults to `fork`). The filter refuses `socket`, `bind`, `listen`
  and `connect` on purpose ("no new endpoint"), so this is expected. Every other program, including
  `asyncio`, `subprocess`, threads and `os.fork`, stayed inside the list.
- **In the real sandbox no program was stopped by the filter** (0 of 31 `SECURITY_VIOLATION`). 29 ended
  as they should. The two `multiprocessing` pool programs exit with 1 and `FileNotFoundError` when they
  create a lock: POSIX semaphores need `/dev/shm`, and the sandbox's `/dev` holds only `null`, `zero`,
  `full`, `urandom` and `random`. That is how the sandbox's file system is built, not the filter, and it means
  `multiprocessing` pools, queues, locks and managers cannot work in the sandbox. It also means the
  `AF_UNIX` socket operations that the comment in `filter.c` gives as needed by "asyncio, multiprocessing"
  are, as measured, needed by `asyncio` (`socketpair`, `epoll`, `pidfd_open`); whether `multiprocessing.Process`
  with a `Pipe` works was not tried.

### The candidates for narrowing named on 12 Nov

| Call | Used by the corpus? |
|---|---|
| `memfd_create` | No |
| `sysinfo` | No |
| `setsockopt` | No (`getsockopt` only by the fork server program) |
| `uname` | Yes: `platform.system()` and `os.uname()` (2 programs); not by interpreter start-up |
| `splice`, `tee`, `sendfile` | No |
| `copy_file_range` | Yes: `shutil.copy` (the file program) |

So `memfd_create`, `sysinfo` and `setsockopt` were not needed by anything in the corpus, while narrowing
`uname` would break `platform` and `os.uname()`. Not being used by 31 programs is not proof that no student
program needs a call, which is why the decision needs the UAT programs and the standard library features
students use, as the audit said.

### Allowed but not used by any program (132 of 205 allowed outright)

alarm, capget, chown, clock_getres, creat, dup, dup3, epoll_create, epoll_pwait, epoll_pwait2, eventfd,
eventfd2, execveat, faccessat, faccessat2, fadvise64, fallocate, fchdir, fchmod, fchmodat, fchmodat2,
fchown, fchownat, fdatasync, fgetxattr, flistxattr, flock, fork, fstatfs, fsync, ftruncate,
futex_requeue, futex_wait, futex_waitv, futex_wake, futimesat, get_robust_list, get_thread_area,
getcpu, getdents, getgroups, getitimer, getpgid, getpgrp, getpriority, getresgid, getresuid, getrlimit,
getsid, gettimeofday, getxattr, lchown, lgetxattr, linkat, listxattr, llistxattr, lstat, membarrier,
memfd_create, mincore, mkdirat, mremap, msync, nanosleep, pidfd_send_signal, pipe, ppoll, preadv,
preadv2, pselect6, pwrite64, pwritev, pwritev2, readahead, readlinkat, readv, recvmmsg, renameat,
renameat2, restart_syscall, rt_sigpending, rt_sigqueueinfo, rt_sigsuspend, rt_sigtimedwait,
rt_tgsigqueueinfo, sched_get_priority_max, sched_get_priority_min, sched_getattr, sched_getparam,
sched_getscheduler, sched_rr_get_interval, sched_yield, select, sendfile, sendmmsg, set_thread_area,
setpgid, setrlimit, setsid, setsockopt, shutdown, sigaltstack, signalfd, signalfd4, splice, stat, statfs,
statx, symlink, symlinkat, sync_file_range, sysinfo, tee, tgkill, time, timer_create, timer_delete,
timer_getoverrun, timer_gettime, timer_settime, timerfd_create, timerfd_gettime, timerfd_settime,
times, tkill, truncate, umask, utime, utimensat, utimes, waitid, writev.

Many of these are the other names of a call glibc chose differently on this system (`stat` and `statx` for
`newfstatat`, `fork` for `clone`, `time` for `clock_gettime`), and the older glibc on the campus server
may pick some of them; that is another reason to repeat the run there.

## Campus server run (14 Nov 2026)

The same corpus, repeated on the campus server (Ubuntu 24.04, kernel 6.8.0-139-generic, Python 3.12.3,
strace 6.8) with the commands under "Repeating it".

- **71 distinct calls**: 66 on the allowlist outright, the four with an argument rule (`clone`, `ioctl`,
  `prctl`, `socketpair`) and `clone3`, answered with `ENOSYS` by design. **None was outside the list.** The five
  calls the WSL run found outside it (`socket`, `bind`, `listen`, `connect`, `accept4`) came from the
  `multiprocessing` fork server of Python 3.14, and Python 3.12 does not use it by default, which is consistent
  with their absence here.
- **In the real sandbox, 0 of 31 programs were stopped by the filter.** The two `multiprocessing` pool programs
  exit with 1 and `FileNotFoundError`, as in the WSL run (the sandbox's `/dev` has no `/dev/shm`), so pools,
  queues and locks of `multiprocessing` cannot work on the server either. The programs that end with an error
  ended as they should: `uncaught_exception` with exit 1 and the `ZeroDivisionError` traceback,
  `assert_fail` with exit 1 and its `AssertionError`, `exit_code` with 3.
- **One argument check was flagged.** Under `strace`, outside the sandbox, the two programs that end with an
  exception (`uncaught_exception`, `assert_fail`) make `prctl(PR_CAPBSET_READ, ...)` calls (six each), an option
  that is not among the `prctl` options the filter allows. In the sandbox the same two programs were not
  stopped and their tracebacks were reported. This run does not show why: whether the calls are not made inside
  the sandbox or are made and their refusal changes nothing was not examined, and neither was what makes
  Python call it.
- **139 of the 205 calls allowed outright were not used** (132 in the WSL run). Of the candidates named on
  12 Nov, `memfd_create`, `sysinfo`, `setsockopt`, `splice` and `tee` were not used, and `uname` was. The two
  machines choose different calls for the same job: `copy_file_range` was used in the WSL run and not here,
  where `sendfile` was (the file-copy program is the likely user, not checked), so narrowing by the WSL list
  alone would have been wrong for the server. The allowlist is unchanged.

## What this does not show

- Another Python, C library or kernel: only the WSL and the campus server combinations were run.
- Programs students really write: the corpus is mine. The UAT programs are a better test of the
  standard-library surface and have not been run through this.
- Calls made by native code reached through `ctypes` or an extension module, other than loading
  `ctypes` and calling `getpid`.
- Error paths and rarely taken branches of the standard library.
- Other architectures (the call names are x86-64).

## Repeating it

On any Linux host with `strace` (on the campus server, `sudo apt install strace` first), from the
repository:

```sh
python3 sandbox/tests/syscall_census.py strace --json /tmp/census.json
```

In the sandbox, with a built `sandbox-exec` (the same way `tests/run.sh` runs the adversarial suite):

```sh
sandbox/tests/run.sh /abs/path/to/sandbox-exec --exec python3 sandbox/tests/syscall_census.py sandbox
```

The parts of the script that decide its answer have their own tests:
`python3 -m unittest tests/test_syscall_census.py`, run from `sandbox/` (CI does not run them).
