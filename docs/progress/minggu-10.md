# Week 10 progress (20 Oct - 26 Oct 2026)

**Phase / sprint:** Backend critical path to submission flow and UAT preparation

## Done

- [Issue #58](https://github.com/MelvynFaith/sandboxed-autograder/issues/58) tracks B3 (FR-02, FR-03, FR-06; Must). The branch was rebased onto the latest `develop`; [PR #59](https://github.com/MelvynFaith/sandboxed-autograder/pull/59) is open with review requested. Owner: Benedictus Erlangga.
- B3 covers assignment CRUD, resource limits, test-case lifecycle, stale-grade marking, and exclusion of soft-deleted cases from grading. Assignment and test-case write access now returns 404 for a non-owner, including an owner demoted to `STUDENT`.
- Local scheduler gates passed: `gofmt -l .`, `go vet ./...`, and `go test -race -count=1 -v ./...` with `scheduler/.env`. Coverage: API 72.1%, auth 72.9%, and courses 77.1%.
- The demoted-owner test fails when the lecturer-role predicate is removed from either assignment or test-case ownership SQL; both mutations were restored and checked.

## In progress / carried over

- B3 review and merge of [PR #59](https://github.com/MelvynFaith/sandboxed-autograder/pull/59) are required before starting B4 (submission API, FR-04, FR-08, FR-09, FR-10).
- Confirm ownership and contracts for FR-14 rate limiting, WebSocket tickets/streaming, regrade, FR-16 submission deletion, and FR-20 before B4 work.
- Confirm the UAT window of 27 Oct–9 Nov with Pak Peter; no confirmation has been recorded yet.
- Determine worker behavior for assignments without a configured resource limit; `GET /resource-limit` currently returns 404.

## Blockers and risks

- B4 depends on B3 being reviewed and merged; the end-to-end upload, grading, and report flow remains blocked until then.
- R6: the UAT schedule confirmation is overdue. The 27 Oct–9 Nov window is still a proposal, not a confirmed schedule.
- Feature freeze is 26 Oct. Prioritize B4 and UAT-critical requirements over unstarted frontend work if the schedule slips.

## Next week

- Get B3 reviewed, address feedback, and merge only after approval and green CI.
- Start B4 from updated `develop` after B3 merges, using the agreed queue and endpoint contracts.
- Confirm the UAT schedule and prepare role-based test accounts and assignment scenarios.
