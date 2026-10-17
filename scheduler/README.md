# API Gateway & Job Scheduler

A Go service that exposes the REST and WebSocket API, queues grading jobs, runs the worker pool, and
persists results. It is **the only component that writes to PostgreSQL.**

**Owner:** Evan William
**Status:** authentication/session endpoints and the in-process `JobQueue` are implemented. The
assignment, submission, grading, and WebSocket upgrade APIs are not implemented.

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
cmd/autograder/    API process entry point
migrations/        schema, one NNNNNN_name.up.sql / .down.sql pair per change, embedded as migrations.FS
internal/api/      HTTP routes and handlers
internal/auth/     authentication, password/token helpers, PostgreSQL user/session store, WS tickets
internal/queue/    JobQueue interface and in-process Memory implementation
```

The API/authentication packages are separate from grading work in `internal/queue`. The current
auth/session endpoints do not enqueue grading jobs.

## Authentication API

Run the API with Go 1.27.2 or newer. Configuration is supplied through environment variables:

```sh
export DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/DATABASE?sslmode=disable'
export JWT_SECRET='at-least-32-random-bytes-from-a-local-secret-manager'
export HTTP_ADDR=':8080'
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
  The WebSocket upgrade endpoint that consumes tickets is outside this API increment.
- `GET /admin/users?page=N&page_size=N` returns `items`, `page`, `page_size`, and `has_next`.
  `PUT /admin/users/{id}/role` accepts `{ "role" }`. Only `ADMIN` can use these routes; role changes
  revoke every refresh token for the target user and write a `ROLE_CHANGED` audit row in the same
  transaction. A non-admin role-change attempt returns 403 and writes `ROLE_CHANGE_DENIED`.
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

## Build and test

Needs Go 1.27 or newer. The migration tests need a PostgreSQL server; without one they are skipped.

```sh
cd scheduler
go vet ./...
go test -race ./...       # migration tests skipped unless the variable below is set
go build ./...
```

```sh
export AUTOGRADER_TEST_DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/DATABASE?sslmode=disable'
```

Each test creates its own schema in that database and drops it afterwards, so a scratch database is
enough; use a user that may create schemas. CI runs PostgreSQL 16, the version Ubuntu 24.04
packages, and fails instead of skipping when the variable is missing.

The API process uses `DATABASE_URL`, `JWT_SECRET` (at least 32 bytes), and `HTTP_ADDR`; tests use
`AUTOGRADER_TEST_DATABASE_URL` and apply migrations in isolated schemas.
