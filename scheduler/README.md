# API Gateway & Job Scheduler

A Go service that exposes the REST and WebSocket API, queues grading jobs, runs the worker pool, and
persists results. It is **the only component that writes to PostgreSQL.**

**Owner:** Evan William
**Status:** foundation only. The schema migrations and the `JobQueue` interface exist; the queue
implementation, worker pool and HTTP API do not.

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
migrations/        schema, one NNNNNN_name.up.sql / .down.sql pair per change, embedded as migrations.FS
internal/queue/    the JobQueue interface between the HTTP handlers and the worker pool
```

Code that accepts requests (an `internal/api` package, not written yet) and code that grades
(`internal/queue` and the worker) stay in separate packages. They meet at `JobQueue`: a handler stores the submission as `QUEUED` and calls
`Enqueue`; a worker calls `Dequeue`.

## Migrations

The schema follows SRS Bagian 7.1. Every up migration has a matching down migration, and the test
applies both. Rules for a new migration:

- Number it after the last one and write both files; never edit a migration that has been merged.
- Enumerations are `text` with a `CHECK`, so adding a value is a constraint change.
- PostgreSQL `text` cannot hold a NUL byte. Strip it from program output before inserting.
- Deleting a row that an `ON DELETE RESTRICT` key protects raises SQLSTATE `23503`
  (`foreign_key_violation`) on PostgreSQL 16 and `23001` (`restrict_violation`) on 18. Map both
  when turning database errors into API errors.

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
