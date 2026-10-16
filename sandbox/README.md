# Sandbox Executor (`sandbox-exec`)

A C binary that runs **one command** in an isolated, resource-limited environment and reports how it
ended. The grading harness calls it once per test case. It knows nothing about test cases or scores.

**Owner:** Efraim Melvyn Rafelino Lahai · **Backup:** Evan William
**Status:** implemented and tested on Linux 6.x (WSL2). Open items are under [Known gaps](#known-gaps).

The binding interface (command line, report fields, exit status, mapping to `Result.status`) is
SRS Bagian 3.2.1.7. This file covers building, running and operating the binary.

## What it does

- Clones the child with `clone3()` into new user, PID, mount, network, UTS and IPC namespaces, as an
  unprivileged service user. The binary is never root and never setuid.
- Puts the child in its own cgroup v2 sub-cgroup: `memory.max`, `memory.swap.max=0`,
  `memory.oom.group=1`, `pids.max`, and `cpu.max` for bandwidth only.
- Enforces the CPU-time budget (`cpu.stat`, millisecond precision, `RLIMIT_CPU` as a backstop), the
  wall-clock timeout, the combined stdout+stderr cap and memory sampling from one `poll()` loop.
- Builds a minimal root with `pivot_root`: read-only `/usr` (plus `/bin`, `/lib`, `/lib64`),
  `/etc/ld.so.cache` only, a small `/dev`, a fresh `/proc`, and a size-capped tmpfs at `/work`.
- Loads a seccomp-BPF **allowlist**. Any other syscall is `SCMP_ACT_NOTIFY`: the supervisor reads
  its number, refuses it, and ends the run as `SECURITY_VIOLATION`.
- Reports exit status, signal, times, peak memory and truncation from `wait4()` and cgroup counters,
  on a channel the program cannot write to (the supervisor's stdout, or `--result FILE`).

Per-run overhead is about 3.5 ms on WSL2 (`clone3` 1.2 ms, seccomp 0.7 ms, cgroup 0.3 ms).

## Build

Needs GCC or Clang, `make`, `pkg-config` and `libseccomp-dev` (≥ 2.5).

```sh
make            # ./sandbox-exec, built with -Wall -Wextra -Werror (what CI runs)
make test       # adversarial suite
make asan       # AddressSanitizer + UBSan build, then the suite
make tidy       # clang-tidy
make install    # binary and man page under $(DESTDIR)$(PREFIX), default /usr/local
```

The reference for options, the JSON report, termination reasons and exit status is the man page:
`man ./sandbox-exec.1`, or `man sandbox-exec` after `make install`.

Without a system-wide `libseccomp-dev`: `make SECCOMP_CFLAGS=-I/path/include SECCOMP_LIBS=-l:libseccomp.so.2`.

Code follows the Linux kernel coding style (tabs, 100 columns); `.clang-format` encodes it.

## Host requirements

- Linux ≥ 5.10 with the cgroup v2 unified hierarchy. `memory.peak` (5.19), `cgroup.kill` (5.14) and
  `mount_setattr` (5.12) are used when present and have fallbacks.
- seccomp user notification (kernel ≥ 5.0, libseccomp ≥ 2.5), `clone3` (5.3), `pidfd_getfd` (5.6),
  `close_range` (5.9).
- Unprivileged user namespaces. On Ubuntu 24.04+ set `kernel.apparmor_restrict_unprivileged_userns=0`
  or ship an AppArmor profile for `sandbox-exec` (SRS risk R9); otherwise `mount` fails with `EPERM`.
- Tested with Yama `ptrace_scope=1` (the supervisor is the child's ancestor, which `pidfd_getfd`
  needs). Stricter values were not tested.

### cgroup delegation

`--cgroup-root` must be a cgroup the service user owns, with `memory`, `pids` and `cpu` enabled in
`cgroup.subtree_control` and **no processes of its own** (cgroup v2 "no internal process" rule).
`sandbox-exec` itself must run below the same delegated cgroup, because moving the child needs write
access to `cgroup.procs` of the common ancestor. For a systemd unit:

```ini
[Service]
User=grader
Delegate=yes
```

At worker start-up, before the first job: create a leaf (`<unit cgroup>/worker`), move the worker
process into it, write `+memory +pids +cpu` to `<unit cgroup>/cgroup.subtree_control`, and pass
`<unit cgroup>` as `--cgroup-root`. `tests/run.sh` does the same with a transient
`systemd-run --user --scope -p Delegate=yes`.

Run `sandbox-exec --sweep --cgroup-root DIR --run-dir DIR` once at worker start-up to remove what
a crashed run left behind.

## Use

```sh
printf '5\n' | sandbox-exec --cgroup-root "$CG" \
    --cpu-time-ms 2000 --timeout-s 5 --memory-mb 64 --max-processes 5 \
    --max-output-bytes 1048576 --tmpfs-size-mb 32 \
    --copy solution.py:main.py -- /usr/bin/python3 -s -P -B main.py
```

stdin goes to the program and the JSON report comes back on stdout. Exit status: `0` ran, `2` usage
error or limit invariant violated, `70` internal error. The invariants are
`2 × max_output_bytes < tmpfs_size_mb × 2^20` and `tmpfs_size_mb + 16 ≤ memory_mb`.

The interpreter flags are `-s -P -B`, not `-I`: `-I` implies `-E`, which makes Python ignore
`PYTHONHASHSEED` and brings back run-to-run differences in `set` ordering.

## Tests

`tests/adversarial.py` (standard library only) covers the SRS Bagian 11.3 scenarios: fork, CPU,
wall-clock, memory (also with swap), output and disk-write bombs, path traversal, network and
dangerous syscalls (with their numbers), result forgery, input visible in `/proc/<pid>/cmdline`,
determinism, crash cleanup and `--sweep`, parallel runs, and usage errors. `tests/run.sh` prepares the
delegated cgroup; `tests/run.sh BIN --exec CMD...` runs any command in it.

```sh
make test
bash tests/run.sh "$PWD/sandbox-exec" -k fork_bomb     # one test, unittest syntax
```

CI only builds this component: the suite needs user namespaces and cgroup v2, which GitHub-hosted
Ubuntu 24.04 runners restrict (SRS R9). Run it on a self-hosted runner.

### Extending the allowlist

The list is in `src/filter.c`. A legitimate program stopped as `SECURITY_VIOLATION` shows the syscall
number in the report. To see everything a program needs, run it outside the sandbox with
`strace -f -c python3 -s -P -B prog.py`. Adding a syscall is a security-sensitive change: say in the
review what it exposes.

## Known gaps

- No `SCMP_ACT_TRACE` fallback for kernels without user notification (SRS risk R12). There the
  program refuses to run (`seccomp_load` fails) instead of running unfiltered.
- `multiprocessing` (`Pool`, `Lock`) fails: POSIX semaphores need `/dev/shm`, which is not mounted.
  `threading`, `asyncio`, `subprocess` and `os.fork` work.
- The program is PID 1 of its namespace, so a signal it sends to itself with `kill()` is ignored by
  the kernel and the run ends at the wall-clock limit as `TIMEOUT`.
- Stdlib-only is policy, not enforcement: system packages under `/usr/lib/python3/dist-packages` stay
  importable. Keep the harness dependencies outside `/usr` (a virtualenv in `/opt`). On WSL2,
  `/usr/lib/wsl` is visible, read-only, inside the sandbox.
- `clang-tidy` has not been run (not installed on the development machine). The code builds with
  `-Wall -Wextra -Werror -Wshadow -Wcast-qual`, and the suite passes under ASan/UBSan and with the
  `mount_setattr` fallback forced.

## Layout

```
Makefile
sandbox-exec.1    man page
.clang-format     kernel coding style
src/sbx.h         declarations shared by all modules
src/main.c        argument handling, stdin, report, exit status
src/opts.c        command line and the tmpfs/memory invariants
src/run.c         helper namespace, run directory, tmpfs, clone3, cleanup, --sweep
src/child.c       PID 1 side: mounts, pivot_root, rlimits, capabilities, seccomp, execve
src/filter.c      seccomp allowlist
src/supervisor.c  watchdog loop, seccomp notifications, stdin, result collection
src/cgroup.c      sub-cgroup creation, limits, counters, sweep
src/report.c      JSON writer
tests/            adversarial suite and cgroup wrapper
```
