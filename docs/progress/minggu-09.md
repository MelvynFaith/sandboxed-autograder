# Week 09 progress (13 Oct - 19 Oct 2026)

**Phase / sprint:** Backend readiness, frontend integration, and UAT preparation

## Done

- [PR #19](https://github.com/MelvynFaith/sandboxed-autograder/pull/19) — Frontend SRS alignment, admin/audit panel, and review fixes merged to `develop` on 15 Oct. Owner: Benedictus Erlangga.
- Frontend lint, 64 tests, and production build passed locally; the PR frontend CI check passed.
- On 17 Oct, these changes were merged to `develop`:
  - [PR #23](https://github.com/MelvynFaith/sandboxed-autograder/pull/23) — receive one seccomp notification per poll. Owner: Melvyn Faith.
  - [PR #24](https://github.com/MelvynFaith/sandboxed-autograder/pull/24) — AppArmor profile and systemd unit for the sandbox host. Owner: Melvyn Faith.
  - [PR #25](https://github.com/MelvynFaith/sandboxed-autograder/pull/25) — Go module, initial schema migration, and JobQueue interface. Owner: Melvyn Faith.
  - [PR #26](https://github.com/MelvynFaith/sandboxed-autograder/pull/26) — grader project skeleton. Owner: whosblub.
  - [PR #27](https://github.com/MelvynFaith/sandboxed-autograder/pull/27) — in-process JobQueue. Owner: Melvyn Faith.
  - [PR #29](https://github.com/MelvynFaith/sandboxed-autograder/pull/29) — Grading Harness runner. Owner: Melvyn Faith.
  - [PR #31](https://github.com/MelvynFaith/sandboxed-autograder/pull/31) — worker pool. Owner: Melvyn Faith.
  - [PR #32](https://github.com/MelvynFaith/sandboxed-autograder/pull/32) — frontend registration and profile flows. Owner: Benedictus Erlangga.
  - [PR #33](https://github.com/MelvynFaith/sandboxed-autograder/pull/33) — output comparison. Owner: whosblub.
  - [PR #36](https://github.com/MelvynFaith/sandboxed-autograder/pull/36) — submission rate limits. Owner: Melvyn Faith.
- B0 local backend preparation: Go 1.27.2 and PostgreSQL 18.6 are available, initial migrations were applied, and `gofmt`, `go vet`, and `go test -race` passed. Local development credentials remain in the ignored `scheduler/.env`.
- The CRUD/pipeline boundary is documented in the scheduler layout: scheduler owns HTTP/API orchestration and is the only PostgreSQL writer; grader and sandbox own submitted-code execution. The `JobQueue` boundary carries `SubmissionID` and `UserID`.

## In progress / carried over

- B1 auth/session API (WBS 3.4; FR-01, FR-15, FR-19) is on [PR #35](https://github.com/MelvynFaith/sandboxed-autograder/pull/35). The conflicts are resolved and CI is green; Melvyn's re-review is pending. Coverage with `-coverpkg=./...`: `internal/api` 70.7%, `internal/auth` 76.4%, total 76.7%. B1 is not merged.
- Open scheduler work: [PR #37](https://github.com/MelvynFaith/sandboxed-autograder/pull/37) (settings table), [PR #38](https://github.com/MelvynFaith/sandboxed-autograder/pull/38) (sweep dead sandbox runs at start-up), and [PR #39](https://github.com/MelvynFaith/sandboxed-autograder/pull/39) (give up on repeatedly lost submissions). Owners: Melvyn Faith.
- Implement the FR-11 grade-summary/CSV frontend UI; confirm its backend endpoint contract and backend owner.
- B2 Course and enrollment (FR-18) depends on the auth middleware/helpers in B1. Start it from updated `develop` after PR #35 merges, not from the B1 branch.

## Blockers and risks

- R6: the final UAT schedule confirmation deadline (12 Oct, end of Week 8) has passed. The schedule is **not confirmed**, and there is no evidence that a message to the instructor has been sent. The proposed UAT window is 27 Oct–9 Nov. The SRS fallback is students from another class participating under instructor supervision; the PM, Benedictus Erlangga, decides whether to use it.
- R4 and R7: Evan has left; he was the planned backup for the sandbox module and a reviewer for Module C. This reduces sandbox backup and review capacity; replacement coverage has not been confirmed.
- Feature freeze is 26 Oct and UAT starts 27 Oct. The team still needs to decide which Should/Could requirements can be deferred and confirm SRS ownership and API contracts.

## Next week

- Obtain Melvyn's re-review before merging B1; then start B2 (FR-18) from updated `develop`.
- Confirm replacement sandbox backup and Module C reviewer coverage (R4, R7).
- Have the PM decide whether to use the SRS UAT fallback (R6); document a confirmed schedule only after confirmation.
- Agree Should/Could scope cuts before the 26 Oct feature freeze, and confirm the backend owner/API contract for FR-11.
