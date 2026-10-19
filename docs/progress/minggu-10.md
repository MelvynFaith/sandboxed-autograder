# Week 10 progress (20 Oct - 26 Oct 2026)

**Phase / sprint:** Backend critical path to submission flow and UAT preparation

## Done

- [Issue #58](https://github.com/MelvynFaith/sandboxed-autograder/issues/58) tracks B3 (FR-02, FR-03, FR-06; Must). The branch was rebased onto the latest `develop`; [PR #59](https://github.com/MelvynFaith/sandboxed-autograder/pull/59) is open with review requested. Owner: Benedictus Erlangga.
- [Issue #60](https://github.com/MelvynFaith/sandboxed-autograder/issues/60) tracks the remaining FR-03 Must work for YAML test-case import/export; it is not included in PR #59.
- B3 covers assignment CRUD, resource limits, test-case lifecycle, stale-grade marking, and exclusion of soft-deleted cases from grading. Assignment creation now requires and atomically stores all six resource-limit fields. Test cases are soft-deleted whenever their assignment has any submission, including a running submission; assignment/test-case writes return 404 for non-owners, including an owner demoted to `STUDENT`.
- After the review fixes, `gofmt -l .` returned no files, `go vet ./...` passed, and `go test -race -count=1 -v ./...` passed with `scheduler/.env`; the suite had one intentional skip (`TestHelperProcess`). Coverage was measured through API integration tests by running `go test -coverpkg=./internal/api -cover ./internal/api -count=1`, then repeating with `./internal/auth` and `./internal/courses`; results were API 72.0%, auth 72.9%, and courses 76.7%.
- The demoted-owner test fails when the lecturer-role predicate is removed from either assignment or test-case ownership SQL; both mutations were restored and checked.
- Review-fix mutation checks also passed: removing create-time resource-limit invariant validation made the invalid-limit API tests fail; reverting test-case deletion to check existing results made the no-result and `RUNNING` tests fail; disabling the hidden-case filter exposed the hidden item and failed the student assertion. Each mutation was restored and verified with `git diff --exit-code`.

## In progress / carried over

- B3 review and merge of [PR #59](https://github.com/MelvynFaith/sandboxed-autograder/pull/59) are required before starting B4 (submission API, FR-04, FR-08, FR-09, FR-10).
- Confirm ownership and contracts for FR-14 rate limiting, WebSocket tickets/streaming, regrade, FR-16 submission deletion, and FR-20 before B4 work.
- Confirm the UAT window of 27 Oct–9 Nov with Pak Peter; no confirmation has been recorded yet.
- Finish the requested changes on PR #59; do not start B4 until B3 is merged.

## Blockers and risks

- B4 depends on B3 being reviewed and merged; the end-to-end upload, grading, and report flow remains blocked until then.
- R6: the UAT schedule confirmation is overdue. The 27 Oct–9 Nov window is still a proposal, not a confirmed schedule.
- Feature freeze is 26 Oct. Prioritize B4 and UAT-critical requirements over unstarted frontend work if the schedule slips.

## Next week

- Get B3 reviewed, address feedback, and merge only after approval and green CI. Implement the separate YAML Must issue before declaring FR-03 complete.
- Start B4 from updated `develop` after B3 merges, using the agreed queue and endpoint contracts.
- Confirm the UAT schedule and prepare role-based test accounts and assignment scenarios.
