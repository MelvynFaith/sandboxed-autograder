# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Repository scaffolding: README, MIT license, contribution guide, security policy, and component
  directories.
- GitHub Actions CI. Each component's jobs activate once that component contains code.
- Issue templates for bug reports and user stories, and a pull request template.
- Weekly progress and sprint review templates under `docs/`.
- Sandbox Executor (`sandbox/`): `sandbox-exec` runs one command in user, PID, mount, network, UTS
  and IPC namespaces with cgroup v2 limits, a seccomp allowlist that reports the denied syscall
  number, a deterministic environment and a JSON result channel, plus `--sweep` for crash leftovers
  and an adversarial test suite (`make test`, `make asan`). Ships a man page and `make install`.
- Sandbox Executor (`sandbox/`): a `ptrace` fallback for kernels without seccomp user notification (SRS
  risk R12). The filter then uses `SCMP_ACT_TRACE` and the supervisor traces the program; results are
  unchanged. `make trace` runs the whole suite on it. `clone()` with `CLONE_UNTRACED` is now denied.
- Sandbox host setup (`infra/`): an AppArmor profile that lets `sandbox-exec` create user
  namespaces on Ubuntu 24.04, a systemd unit with cgroup delegation, and install and verification
  steps.
- Scheduler foundation (`scheduler/`): Go module, the initial PostgreSQL schema as a golang-migrate
  migration pair with a test that applies, reverses and checks its constraints, and the `JobQueue`
  interface with its in-process implementation, which serves students round-robin (FR-13). The
  scheduler CI job now starts a PostgreSQL service.
- Scheduler authentication and sessions (`scheduler/`): Argon2id registration/login, JWT access
  tokens, hashed and revocable refresh tokens, self-profile and password management, single-use
  WebSocket tickets, and paginated administrator user management (FR-01, FR-15, FR-19).
- Scheduler auth review updates (`scheduler/`): audit administrator role changes and denied
  attempts; bound Argon2id concurrency; serialize refresh rotation/revocation against user
  credential changes; enforce 200-character display names and equalize unknown-email login work.
- Grading pipeline (`scheduler/`): the Grading Harness runner, which keeps the assignment definition
  off the command line and checks the report; the grading store, which moves a submission through
  `QUEUED`, `RUNNING` and `COMPLETED` or `FAILED` and writes results and audit rows in one
  transaction; and the worker pool, with panic recovery, a backstop deadline and reconciliation of
  lost submissions.
