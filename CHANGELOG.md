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
- First administrator (`scheduler/`): `autograder seed-admin --name NAME --email EMAIL` creates the
  first `ADMIN` account with the password from `ADMIN_PASSWORD`, and an `ADMIN_SEEDED` audit row; it
  refuses once an administrator exists (FR-19, SRS 13.2).
- Grading progress over WebSocket (`scheduler/`): `/ws/v1/submissions/{id}` streams queued, running
  test n of total and the final status to the dashboard, authenticated by the one-time ticket
  (FR-08). The Harness may report each test case it starts on `HARNESS_PROGRESS_FD`.
- Scheduler authentication and sessions (`scheduler/`): Argon2id registration/login, JWT access
  tokens, hashed and revocable refresh tokens, self-profile and password management, single-use
  WebSocket tickets, and paginated administrator user management (FR-01, FR-15, FR-19).
- Scheduler auth review updates (`scheduler/`): audit administrator role changes and denied
  attempts; bound Argon2id concurrency; serialize refresh rotation/revocation against user
  credential changes; enforce 200-character display names and equalize unknown-email login work.
- Scheduler health check (`scheduler/internal/health`): the `/healthz` handler reports database
  reachability, live workers and queue depth, and answers `503` when the database is down or the
  worker pool is not at full strength (NFR-09).
- Scheduler courses and enrollment (`scheduler/`): add paginated, role-scoped course lists, course
  creation, enrollment management, and course-scoped assignment listing with object-level access
  checks (FR-18, NFR-10).
- Scheduler course ownership review: require an active lecturer owner for every new course, verify
  administrator-selected lecturer emails transactionally, deny demoted owners access, and distinguish
  unknown-user errors from unavailable-course errors.
- Scheduler assignments (`scheduler/`): add role-scoped assignment CRUD, resource-limit validation,
  paginated test cases with hidden-case filtering, external-ID generation, stale-grade marking, and
  submission-aware test-case deletion.
- Assignment creation now requires all six resource-limit values in `resource_limit` and creates
  the assignment and its one-to-one resource limit atomically, as required by SRS 7.1. This changes
  the earlier API contract: clients must send resource limits when creating an assignment; PUT
  resource-limit only updates the existing row.
- Scheduler submissions (`scheduler/`): accept enrolled-student Python uploads up to 256 KiB,
  enforce assignment deadlines and configured FR-14 rate limits transactionally, save and enqueue
  `QUEUED` jobs, provide paginated student history and scoped submission details, and redact hidden
  test-case data from student reports.
- Grading pipeline (`scheduler/`): the Grading Harness runner, which keeps the assignment definition
  off the command line and checks the report; the grading store, which moves a submission through
  `QUEUED`, `RUNNING` and `COMPLETED` or `FAILED` and writes results and audit rows in one
  transaction; and the worker pool, with panic recovery, a backstop deadline, reconciliation of
  lost submissions, and a sweep of what dead sandbox runs left behind.
- Scheduler process (`scheduler/cmd/autograder`): applies the migrations at start-up, prepares the
  delegated cgroup, and runs the worker pool next to the API, with one shutdown for both. Grading is
  configured through `WORKERS`, `CODE_DIR`, `HARNESS_COMMAND` and `SANDBOX_EXEC`; `WORKERS=0` runs the
  API alone. Logs are JSON lines.
- Backend install steps (`infra/README.md`): PostgreSQL with peer authentication, the Harness in its
  own virtual environment, the environment file and the unit, as run on the campus server; and
  `infra/smoke-test.sh`, which pushes three submissions through the installed pipeline and checks
  their scores, the audit rows, `/healthz` and the leftover cgroups.
- Database backup (`infra/`): a systemd timer that runs `pg_dump` every day at 02:30 WIB as the
  `postgres` user and keeps the dumps of the last 14 days, with the steps to check a backup by
  restoring it and to restore the database (SRS 13.4, NFR-13).
- Load test (`infra/loadtest/`): `seed.sh` registers up to 999 student accounts through the API and
  enrolls them in a load-test course, then reports, resets and removes the load-test submissions.
  The k6 scripts measure each submission until its final WebSocket frame (SRS 11.4): `submit.js`
  against NFR-01a at 5, 10 and 30 accounts and NFR-01b at 60 and 100, `ratelimit.js` against FR-14
  with 200 submissions in a row from one account, and `fairness.js` against FR-13 with one account
  flooding the queue.
- Submission file expiry (`scheduler/`, `infra/`): `autograder expire-code`, run daily by a systemd
  timer, deletes the stored files of submissions whose assignment closed more than a year ago and
  keeps their grades, recording the deletion in the new `submissions.code_deleted_at` (NFR-13,
  SRS 13.4). `--dry-run` counts them.
