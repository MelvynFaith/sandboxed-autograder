# Sandbox Executor (`sandbox-exec`)

A C binary that runs **one command** in an isolated, resource-limited environment and reports how it
ended. The grading harness calls it once per test case. It knows nothing about test cases or scores.

**Owner:** Efraim Melvyn Rafelino Lahai · **Backup:** Evan William
**Status:** not started. Sprint 1 target: run a simple Python program under CPU and memory limits.

## Responsibilities

- Create the child in new user, PID, mount, network, UTS, and IPC namespaces, running as an
  unprivileged service user. The binary is never root and never setuid.
- Place the child in its own cgroup v2 sub-cgroup with `memory.max`, `memory.swap.max=0`,
  `pids.max`, and `cpu.max`.
- Enforce the CPU-time budget (`RLIMIT_CPU` plus `cpu.stat`) and the wall-clock timeout
  independently, from a single supervisor loop.
- Build a minimal root filesystem with `pivot_root`: read-only `/usr` and `/lib`, a minimal `/dev`,
  a fresh `/proc`, and a size-capped `tmpfs` working directory.
- Install a seccomp-BPF allowlist. Denied syscalls use `SCMP_ACT_NOTIFY` so the supervisor can
  report the syscall number.
- Report exit status, signal, time, peak memory, and truncation on a channel the student program
  cannot write to.
- Remove the sub-cgroup and working directory after each run, and sweep leftovers at startup.

## Build and test

To be added with the first implementation. CI activates the `sandbox` job once a `Makefile`
exists here. The Makefile must build with `-Wall -Wextra -Werror`.
