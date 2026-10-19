# API Gateway & Job Scheduler

A Go service that exposes the REST and WebSocket API, queues grading jobs, runs the worker pool, and
persists results. It is **the only component that writes to PostgreSQL.**

**Owner:** Evan William
**Status:** authentication/session, course/enrollment, assignment, resource-limit, and test-case
endpoints are implemented, as are submission upload/history/report endpoints, the in-process
`JobQueue`, Harness runner, grading store, worker pool, `/healthz`, and WebSocket progress stream.

## Responsibilities

- Serve the REST API under `/api/v1` with JWT access tokens and revocable refresh tokens, and serve
  real-time submission status over WebSocket with one-time tickets.
- Enforce object-level authorization. A request for another user's or another course's object gets
  `404`.
- Queue jobs behind the `JobQueue` interface. The in-process worker pool uses round-robin fairness
  per student and per-student rate limits.
- Run the grading harness once per submission, passing the assignment definition as a file path,
  never on the command line. Write all results in one transaction.
- Reconcile stale `RUNNING` jobs at startup and expose `/healthz`.

## Layout

```
go.mod
cmd/autograder/    the process: migrations, API server and worker pool
migrations/        schema, one NNNNNN_name.up.sql / .down.sql pair per change, embedded as migrations.FS
internal/api/      authentication and course/assignment HTTP routes and handlers
internal/health/   the /healthz handler: database, live workers, queue depth
internal/auth/     authentication, password/token helpers, PostgreSQL user/session store, WS tickets
internal/courses/  course, enrollment, and course-scoped assignment listing store
internal/submissions/ submission creation, scoped history/report reads, and submission lifecycle storage
internal/queue/    JobQueue interface between the handlers and worker pool; in-process Memory implementation
internal/harness/  starts the Grading Harness for one submission and checks its report
internal/progress/ in-process hub that carries grading progress from the workers to the WebSocket
internal/stream/   the /ws/v1/submissions/{id} WebSocket handler
internal/seed/     creates the first administrator (autograder seed-admin)
internal/retention/ deletes the files of submissions past their retention (autograder expire-code)
internal/store/    grading database operations: claim, complete, fail, and reconcile
internal/worker/   worker pool joining the queue, store, and Harness
internal/sandbox/  delegated cgroup set-up and the start-up sweep of dead sandbox runs
internal/testdb/   isolated migrated schema for each database test
```

## Running

`cmd/autograder` is the whole backend in one process (SRS 3.3). At start-up it applies the
migrations the database does not have yet, prepares the delegated cgroup, starts the worker pool,
and serves the API, `/healthz` and `/ws/v1/submissions/{id}`. On `SIGTERM` it closes the WebSocket
streams with 1001, stops taking requests, interrupts the jobs in progress, which stay `RUNNING` for the
next start-up, and exits. `infra/README.md` lists the environment variables and the
install steps.

The worker pool needs a cgroup delegated to the service user: the binary creates the leaf
`<own cgroup>/worker`, moves itself into it, enables the `memory`, `pids` and `cpu` controllers on its
own cgroup and hands that cgroup to the Harness as `SANDBOX_CGROUP_ROOT`, with the `sandbox-exec` path
as `SANDBOX_EXEC` and nothing else in the Harness environment. The systemd unit delegates with
`Delegate=yes`. For a local run on Linux use a scope of your own:

```sh
systemd-run --user --scope -p Delegate=yes ./autograder
```

Without delegation it exits with an error that names `Delegate=yes`. `WORKERS=0` runs the API alone,
needs no cgroup or Harness, and grades nothing; it still needs `CODE_DIR` to save uploaded submissions.

The queue is in memory and is filled from the database at start-up and every minute after; submission
handlers also enqueue accepted jobs immediately. `WORKERS=0` still accepts and queues submissions in
the database, but does not grade them until a worker process starts.

The grading pipeline includes the schema, in-process queue, Harness runner, grading store, and
worker pool. Request handling stores a submission as `QUEUED` and calls `Enqueue` with its submission
and user IDs; a worker calls `Dequeue`.

## Authentication API

Run the API with Go 1.27.2 or newer. Configuration is supplied through environment variables:

```sh
export DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/DATABASE?sslmode=disable'
export JWT_SECRET='at-least-32-random-bytes-from-a-local-secret-manager'
export HTTP_ADDR=':8080'
export WORKERS=0          # API only; grading needs the settings in "Running"
export CODE_DIR=/tmp/autograder-submissions
go run ./cmd/autograder
```

Do not commit credentials or a `.env` file. Register/login, self-profile, password change, one-time
WebSocket ticket issuance, and administrator user management are implemented under `/api/v1`.
Refresh tokens are opaque random values; only their SHA-256 hashes are stored. Access JWTs use
HS256 and expire after 15 minutes. Password hashes use Argon2id (`m=65536,t=3,p=2`).
At most three password hash or verification operations run concurrently; requests waiting for a slot
can be cancelled with their request context.

The client contract is:

- `POST /auth/register` accepts `{ "name", "email", "password" }`, always creates a `STUDENT`, and
  returns the created user with status 201. A duplicate email returns 409.
- `POST /auth/login` returns `access_token`, `refresh_token`, and `user`.
- `POST /auth/refresh` accepts `{ "refresh_token" }`, revokes it, and returns new access and refresh
  tokens. `POST /auth/logout` accepts `{ "refresh_token" }` and revokes all active refresh tokens
  belonging to that user.
- `GET /users/me` and `PUT /users/me` return the user; the latter accepts `{ "name" }`.
- `PUT /users/me/password` accepts `{ "current_password", "new_password" }`, verifies the old
  password, then revokes every refresh token for the user.
- `POST /ws-tickets` accepts `{ "submission_id" }` and returns `{ "ticket" }`. The request field
  follows `frontend/src/api/submissionStatus.js`; the frontend sends the ID as JSON. Issued tickets
  are held in process memory, bound to the authenticated user and submission, and expire within 60
  seconds. `TicketStore.Consume` atomically removes a ticket; a second or expired use is rejected.
  `/ws/v1/submissions/{id}` consumes them (see "Grading progress").
- `POST /submissions` accepts `multipart/form-data` with `assignment_id` and one `file` part, as
  specified in SRS Appendix B. It returns 202 Accepted with the queued submission. The file must
  have a valid UTF-8 `.py` filename without NUL and be no larger than 256 KiB. The server stores
  the file under a generated name in `CODE_DIR`; the client filename is retained as metadata only.
  An enrolled student may submit through `deadline_at` normally; a later submission through
  `late_until_at` is marked late, and one after that returns 409. Submission insertion and FR-14
  rate-limit checks share one database transaction; rate-limit rejection returns 429. Accepted
  submissions are stored as `QUEUED` and immediately enqueued as `{submission_id, user_id}`.
- `GET /submissions?page=N&page_size=N&assignment_id=ID` returns only the caller's paginated
  history, optionally filtered to one assignment.
  `GET /submissions/{id}` and `GET /submissions/{id}/report` are visible to the submission owner,
  the current lecturer owner of its Course, and administrators; other callers receive 404.
  Reports use the seven per-test verdicts in the schema. Hidden test data is revealed only to
  administrators and current lecturers who own the Course, regardless of the submission owner's
  current role. Other viewers receive only `status`, `score`, `exec_time_ms`, and `memory_kb` for
  hidden results; test identity, input, expected output, actual output, and error message are omitted.
- `GET /admin/users?page=N&page_size=N` returns `items`, `page`, `page_size`, and `has_next`.
  `PUT /admin/users/{id}/role` accepts `{ "role" }`. Only `ADMIN` can use these routes; role changes
  revoke every refresh token for the target user and write a `ROLE_CHANGED` audit row in the same
  transaction. A non-admin role-change attempt returns 403 and writes `ROLE_CHANGE_DENIED`.
- `GET /courses?page=N&page_size=N` returns a paginated course list. Students see enrolled courses,
  lecturers see owned courses, and administrators see all courses. `GET
  /courses/{id}/enrollments` lists enrollments, `POST /courses/{id}/enrollments` adds a student, and
  `DELETE /courses/{id}/enrollments/{user_id}` removes one. These enrollment routes are available to
  the course owner or an administrator. `GET /courses/{id}/assignments?page=N&page_size=N` lists
  existing assignments without providing assignment CRUD; students must be enrolled, while the course
  owner and administrators may list them.
- `POST /courses` accepts `{ "name", "code", "term" }` for a lecturer; the authenticated lecturer
  becomes the owner. An optional `lecturer_email` is accepted only when it matches that lecturer's
  own email. An administrator must supply `{ "lecturer_email" }`; the server verifies in the same
  transaction that the account exists and has role `LECTURER` before creating the course. An unknown
  email returns `404 user_not_found`; a non-lecturer target or a lecturer trying to assign another
  owner returns `400 invalid_request`. FR-18 does not specify these request bodies; `code`, `term`,
  and `lecturer_email` are implementation assumptions matching the schema and role/ownership
  requirements. The API does not generate course fields or alter the merged migration.
- `POST /courses/{id}/enrollments` accepts `{ "email" }`. FR-18 does not specify enrollment body
  fields, so this payload is an implementation assumption matching the frontend course adapter.
  An unknown email returns `404 user_not_found`, distinct from `404 not_found` for an unavailable
  Course. Only the current owner with role `LECTURER` or an administrator can manage enrollments;
  demoted owners lose management access.
- `POST /courses/{id}/assignments` accepts
  `{ "title", "description", "deadline_at", "late_until_at", "resource_limit" }`, with RFC3339
  timestamps, `late_until_at >= deadline_at`, and all six required resource-limit fields:
  `cpu_time_ms`, `memory_mb`, `max_processes`, `timeout_s`, `max_output_bytes`, and `tmpfs_size_mb`.
  Assignment and resource limit are created atomically. The SRS does not define these request
  bodies; this contract follows the assignment schema and SRS 7.1's one-to-one relationship, and
  differs from the earlier API version so clients must send `resource_limit` during creation.
  `PUT /assignments/{id}` accepts `{ "title", "description", "deadline_at", "late_until_at" }`.
- `GET /assignments?page=N&page_size=N` lists assignments visible to the caller. Individual
  assignment reads and writes follow course ownership/enrollment checks. Only a course owner or
  administrator can create, update, or delete assignments.
- `GET` and `PUT /assignments/{id}/resource-limit` read or save
  `{ "cpu_time_ms", "memory_mb", "max_processes", "timeout_s", "max_output_bytes", "tmpfs_size_mb" }`.
  The API validates both sandbox invariants before saving; PUT updates the resource limit created
  with the assignment and does not create one.
- `GET` and `POST /assignments/{id}/testcases`, and `PUT`/`DELETE /testcases/{id}`, manage test
  cases. The write body accepts `input`, `expected_output`, positive `weight`, `is_hidden`,
  `match_mode` (`EXACT` or `WHITESPACE_INSENSITIVE`), and optional `external_id`. An omitted or
  empty `external_id` is generated as `tc_01`, `tc_02`, etc. per assignment; duplicate IDs return
  409. The body shape follows `frontend/src/api/testcases.js`; the SRS does not define it.
- Every test-case create or update marks all submissions for its assignment `stale_grade = true`
  in the same database transaction. Deleting a case soft-deletes it (`is_deleted = true`) whenever
  its assignment has any submission, including a queued or running one; it is physically deleted
  only when the assignment has no submissions. Soft-deleted cases are excluded from API lists and
  future grading.

Registration and profile updates limit display names to 200 Unicode characters.

The SRS does not define the request body fields for registration, password change, or WebSocket
ticket issuance. They follow the current frontend assumptions: register `{name,email,password}`,
password `{current_password,new_password}` (documented in `frontend/README.md`), and ticket
`{submission_id}`. Login/refresh/logout use `email`, `password`, and `refresh_token`, respectively.
If the frontend contract changes, update the adapters and this note together.

Errors use `{"error":{"code":"...","message":"..."}}`. Authentication failures return 401,
authenticated non-admin access returns 403, and a missing or another user's submission returns 404.
Admin user lists accept `page` and `page_size` (maximum 100).

Dependencies added for B1 via `go get`: `github.com/golang-jwt/jwt/v5` (JWT) and
`golang.org/x/crypto` (Argon2id). The existing `pgx/v5` module is used for PostgreSQL access.

## First administrator

Registration always yields a `STUDENT`, and only an administrator can grant `LECTURER` or `ADMIN`
(FR-19), so a new deployment creates its first administrator from the command line (SRS 13.2):

```sh
IFS= read -rsp 'Administrator password: ' ADMIN_PASSWORD && echo && export ADMIN_PASSWORD
sudo --preserve-env=ADMIN_PASSWORD -u grader \
    DATABASE_URL='postgres://grader@/autograder?host=/var/run/postgresql' \
    /usr/local/bin/autograder seed-admin --name 'NAME' --email 'EMAIL'
unset ADMIN_PASSWORD
```

- The password comes only from `ADMIN_PASSWORD`. There is no flag for it, because a flag would show in
  the process list, the shell history and sudo's log; `read -s` keeps it off the screen and out of the
  history, and `--preserve-env` passes it without putting it on a command line.
- It must have 12 to 1024 characters. Name and e-mail follow the registration rules; the e-mail is
  stored in lower case.
- It applies the migrations first, so it works before the service has ever started.
- It writes the account and an `ADMIN_SEEDED` audit log row in one transaction, and refuses, without
  writing anything, if an administrator already exists or the e-mail address is taken. It never
  promotes an existing account, so it cannot become a second way to gain the role: further
  administrators are appointed through `PUT /admin/users/{id}/role`, which is audited.

These are the initial credentials that the handover (SRS 13.5, BAST) passes on.

## Submission file expiry

NFR-13 and SRS 13.4 delete a submission's file one academic year after its assignment closes and
keep its grade for the records. `autograder expire-code`, which `infra/systemd/autograder-expire.timer`
runs daily as `grader`, does that for every submission whose assignment's `late_until_at` is more than
a calendar year ago. `--dry-run` only counts them.

- It needs `DATABASE_URL` and an absolute `CODE_DIR`, and applies the migrations first.
- The row stays: score, results and audit entries are untouched, and `submissions.code_deleted_at`
  records when the file went. A handler that reads the code (FR-20) or queues a regrade (FR-17) should
  check that column and say the file has been deleted; the worker would otherwise fail the regrade
  on the missing file.
- A `QUEUED` or `RUNNING` submission keeps its file. Each row is locked while its file is deleted, so
  a regrade cannot slip in between, and a row locked by someone else waits for the next run.
- A file that is already gone still marks the row. A file that cannot be deleted, or a `code_path` that
  would leave `CODE_DIR`, leaves the row unmarked, is logged, and makes the command exit 1 after the
  other submissions are done.

## Migrations

The schema follows SRS Bagian 7.1. Every up migration has a matching down migration, and the test
applies both. Rules for a new migration:

- Number it after the last one and write both files; never edit a migration that has been merged.
- Enumerations are `text` with a `CHECK`, so adding a value is a constraint change.
- PostgreSQL `text` cannot hold a NUL byte, and a program can print one. `harness.Parse` replaces it
  in the Harness report, so anything that stores output from another source must do the same.
- Deleting a row that an `ON DELETE RESTRICT` key protects raises SQLSTATE `23503`
  (`foreign_key_violation`) on PostgreSQL 16 and `23001` (`restrict_violation`) on 18. Map both
  when turning database errors into API errors.

## Harness

`internal/harness` runs the Grading Harness for one submission and checks what comes back. Its package
comment is the contract between the worker and the Harness, and the Harness owner should read it:

- The worker starts `<command...> --definition PATH --code PATH`. The definition file (test cases with
  their hidden expected output, resource limits, match modes) is written with mode `0600` inside a
  `0700` directory, is never on the command line, and is deleted afterwards.
- The Harness gets an environment of exactly what the deployment configures, not the scheduler's.
- It writes one JSON report (SRS Appendix C) to standard output and exits 0. Any other exit status, or a
  report that does not match the definition (wrong submission, missing or extra test case, a score above
  the weight, totals that do not add up), makes the submission `FAILED` and nothing is stored.
- On a deadline the Harness and the processes it started are killed.
- For the dashboard's progress bar the Harness may write `{"current_test": n}` lines, one before each
  test case, to the file descriptor named in `HARNESS_PROGRESS_FD`. Lines that are malformed or do
  not count up are ignored, and a Harness that writes none still works. `sandbox-exec` marks every
  descriptor above 2 close-on-exec, so student code cannot write to it.

## Settings

The settings an administrator can change (FR-14, US-20) are one row of the `settings` table: the
maximum active submissions, the minimum seconds between submissions, and the maximum submissions per
assignment. The migration creates the row with the FR-14 values (1, 30, 30), so reading never meets a
missing row; 0 switches a rule off. `internal/settings.Store` reads it (`Get`) and saves a change
(`Update`), which validates the bounds and writes a `SETTINGS_CHANGED` audit log row naming the
administrator and the old and new values in the same transaction. The `GET` and `PUT` endpoints belong
to the API package.

## Grading state

`internal/store.Pipeline` is the database side of grading. `Claim` moves a `QUEUED` submission to
`RUNNING` and reads its definition (limits and live test cases); `Complete` stores the results, the
totals and the audit log rows (`SANDBOX_VIOLATION`, `RESOURCE_LIMIT_EXCEEDED`) in one transaction;
`Fail` marks a `RUNNING` submission `FAILED`; `Reconcile` requeues stale `RUNNING` submissions and lists
the `QUEUED` ones, which is how the in-memory queue is rebuilt after a restart. Each state change is a
conditional `UPDATE`, so a cancel, a requeue and a worker cannot push a submission into a state the
diagram in SRS 3.3 does not allow.

A submission that is lost mid-grading, because the scheduler died while it ran, is requeued by
`Reconcile`. `Claim` counts each start in `submissions.attempts`; `Complete` and `Fail` clear the count.
Once a lost submission has been started `Pipeline.MaxAttempts` times (3 by default) `Reconcile` marks it
`FAILED` and logs it instead, so a submission that crashes the scheduler every time cannot loop across
restarts. A regrade starts with a clean count.

## Grading progress (WebSocket)

`GET /ws/v1/submissions/{id}?ticket=TICKET` streams a submission's grading to the dashboard (FR-08,
SRS 6.3). The ticket comes from `POST /api/v1/ws-tickets`, which checks that the submission is the
caller's; it is consumed by the handshake. A bad, expired or reused ticket is refused with `401`, a
ticket for another submission or an unknown submission with `404` (NFR-10). Each frame is the whole
state, in the shape `frontend/src/api/submissionStatus.js` reads:

```json
{"status":"RUNNING","current_test":1,"total_tests":2}
```

- The first frame is the state on connection: the database's (`store.Pipeline.Progress`), or, while
  the submission is being graded, the latest event, which knows the test case.
- The worker pool publishes `RUNNING` test 0 of n when it claims a submission, test k of n as the
  Harness starts each test case, and `COMPLETED` n of n or `FAILED`, each after the database has
  recorded it (`worker.Config.Publish`).
- After a `COMPLETED`, `FAILED` or `CANCELLED` frame the server closes with 1000. On shutdown it closes
  with 1001, and the dashboard falls back to polling. The client sends nothing; the server pings
  every 54 seconds and drops a client silent for 60.
- `progress.Hub` keeps one pending event per connection and replaces it with a newer one, so a slow
  client never holds up a worker; it skips intermediate states, never the last.

Any origin may connect: cross-site WebSocket hijacking rides on cookies, and this endpoint uses none.

## Rate limits

`internal/ratelimit.Limits.Check` enforces FR-14 per student and assignment: at most `MaxActive` (1)
submissions `QUEUED` or `RUNNING`, at least `MinInterval` (30 s) between submissions, and at most
`MaxPerAssignment` (30) in total. A rule set to zero is off. A refusal is an `*ratelimit.Error` naming
the rule (and, for the interval, how long to wait), which the upload handler maps to `429` with a clear
message. Cancelled submissions do not use up the quota (US-14).

The upload handler must call `Check` inside the transaction that inserts the submission: `Check` takes a
lock per student and assignment until the transaction ends, so two simultaneous uploads cannot both pass
a limit of one. The counts come from the database, not memory, so they survive a restart. The thresholds
are admin-configurable (FR-14) but the SRS has no entity to store them, so for now the caller passes a
`Limits` value.

## Worker pool

`internal/worker.Pool` runs `Workers` goroutines, each of which dequeues a job, claims the submission,
runs the Harness and stores the report (SRS 3.3, steps 3 to 8). It is built from three interfaces
(`queue.JobQueue`, `worker.Store`, `worker.Grader`), so it is tested with fakes.

- A job runs under `recover()`. A panic, a failed Harness or an invalid report marks that submission
  `FAILED`, and the worker takes the next job.
- A job has a backstop deadline computed from its definition: each test case may use its `timeout_s`
  plus a small overhead, plus a margin, and never more than half the lease.
- At start-up, and every minute after, `RUNNING` submissions that have not changed for the lease (30
  minutes) go back to `QUEUED`, and `QUEUED` submissions the queue does not hold are added to it
  (NFR-04).
- Before it requeues anything, `Run` calls `Config.Sweep` once. `internal/sandbox.Sweeper` is the
  implementation: it runs `sandbox-exec --sweep` to remove the empty cgroups and run directories that
  a crashed run left behind. A sweep that fails is logged and does not stop the pool.
- On shutdown a job in progress is interrupted and left `RUNNING`, not failed: the next start-up
  requeues it, so a restart does not cost anyone a grade.

## Health check

`internal/health.Checker` serves `GET /healthz` (NFR-09, SRS 6.3.1), outside `/api/v1` and without
authentication, for systemd and Nginx:

```json
{"status":"ok","database":"ok","workers":{"alive":4,"configured":4},"queue_depth":0}
```

It answers `200` with `"status":"ok"` when the database answers a ping within 2 seconds and every
configured worker runs, and `503` with `"status":"unhealthy"` otherwise, for example while the
database is down (`"database":"unreachable"`) or before the pool has finished its start-up
reconciliation. A process started with `WORKERS=0` reports `0` of `0` workers and depends on the
database alone. The body holds these aggregate values only, never an error message; the cause goes
to the log. One check answers every request for a second, so polling cannot take database
connections away from the API. `worker.Pool.Workers` and `queue.Memory.Len` supply the numbers.

## Build and test

Needs Go 1.27 or newer. Tests that touch the database (the migrations and `internal/store`) need a
PostgreSQL server and are skipped without one. `internal/testdb` gives each of them its own migrated
schema.

```sh
cd scheduler
go vet ./...
go test -race ./...       # database tests skipped unless the variable below is set
go build ./...
```

```sh
export AUTOGRADER_TEST_DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/DATABASE?sslmode=disable'
```

Each test creates its own schema in that database and drops it afterwards, so a scratch database is
enough; use a user that may create schemas. CI runs PostgreSQL 16, the version Ubuntu 24.04
packages, and fails instead of skipping when the variable is missing.

The process uses `DATABASE_URL`, `JWT_SECRET` (at least 32 bytes), and `HTTP_ADDR`, plus the grading
variables in `infra/README.md` unless `WORKERS=0`; tests use `AUTOGRADER_TEST_DATABASE_URL` and apply
migrations in isolated schemas.
