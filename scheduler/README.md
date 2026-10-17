# API Gateway & Job Scheduler

A Go service that exposes the REST and WebSocket API, queues grading jobs, runs the worker pool, and
persists results. It is **the only component that writes to PostgreSQL.**

**Owner:** Evan William
**Status:** foundation only. The schema migrations and the in-process `JobQueue` exist; the worker
pool and HTTP API do not.

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
internal/queue/    the JobQueue interface between the HTTP handlers and the worker pool, and Memory,
                   its in-process implementation (round-robin per student, FIFO within a student)
```

Code that accepts requests (an `internal/api` package, not written yet) and code that grades
(`internal/queue` and the worker) stay in separate packages. They meet at `JobQueue`: a handler stores the submission as `QUEUED` and calls
`Enqueue`; a worker calls `Dequeue`.

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

## Grading state

`internal/store.Pipeline` is the database side of grading. `Claim` moves a `QUEUED` submission to
`RUNNING` and reads its definition (limits and live test cases); `Complete` stores the results, the
totals and the audit log rows (`SANDBOX_VIOLATION`, `RESOURCE_LIMIT_EXCEEDED`) in one transaction;
`Fail` marks a `RUNNING` submission `FAILED`; `Reconcile` requeues stale `RUNNING` submissions and lists
the `QUEUED` ones, which is how the in-memory queue is rebuilt after a restart. Each state change is a
conditional `UPDATE`, so a cancel, a requeue and a worker cannot push a submission into a state the
diagram in SRS 3.3 does not allow.

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
