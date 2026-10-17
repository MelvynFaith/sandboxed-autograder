# Week 10 progress (status as of 17 Oct 2026)

**Phase / sprint:** Backend readiness, frontend integration, and UAT preparation

## Done

- B0 local backend preparation is complete: Go 1.27.2 and PostgreSQL 18.6 are available, the
  initial migrations were applied, and `gofmt`, `go vet`, and `go test -race` passed. The local
  development credentials remain in the ignored `scheduler/.env`.
- The current CRUD/pipeline boundary is documented and reflected in the scheduler layout:
  scheduler owns HTTP/API orchestration and is the only PostgreSQL writer; grader and sandbox own
  execution of submitted code. The worker boundary is `JobQueue` (`SubmissionID` and `UserID`),
  so grading work is not coupled to the HTTP handlers.
- B1 authentication/session implementation is on [PR #35](https://github.com/MelvynFaith/sandboxed-autograder/pull/35).
  Local race tests passed with 71.0% total statement coverage; CI checks, including the scheduler
  job against PostgreSQL 16, passed. The PR remains open, awaits Melvyn's review, and is behind
  `develop`.
- F1 registration/profile implementation and review fixes were merged through
  [PR #32](https://github.com/MelvynFaith/sandboxed-autograder/pull/32) on 17 Oct 2026.

## In progress / carried over

- Confirm who owns recording user role changes in `audit_logs` for US-04. The question is posted
  on [issue #34](https://github.com/MelvynFaith/sandboxed-autograder/issues/34); no answer has
  been received yet.
- UAT schedule has not been confirmed; awaiting Pak Peter's response. The proposed date range is
  27 Oct to 9 Nov, and external coordination is handled by the user.

## Blockers and risks

- The scheduler ownership handover after Evan's departure is still unconfirmed.
- B2 (Course and enrollment) depends on auth middleware/helpers from B1. Start B2 from the updated
  `develop` after PR #35 merges, rather than stacking it on the B1 branch.

## Next week

- Follow up on the unconfirmed UAT schedule through the user.
- Obtain Melvyn's review/approval for PR #35; resolve review feedback before merge.
- Confirm the audit-log ownership for role changes.
- After B1 is merged, start B2 from current `develop`.
