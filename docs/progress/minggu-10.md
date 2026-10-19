# Week 10 progress (20 Oct - 26 Oct 2026)

**Phase / sprint:** Backend critical path to submission flow and UAT preparation

## Done

- B3 (FR-02, FR-03, FR-06; Must) merged in [PR #59](https://github.com/MelvynFaith/sandboxed-autograder/pull/59), squash commit `d45c556`; issue #58 is closed.
- [Issue #60](https://github.com/MelvynFaith/sandboxed-autograder/issues/60) tracks the remaining FR-03 Must work for YAML test-case import/export.
- [Issue #61](https://github.com/MelvynFaith/sandboxed-autograder/issues/61) tracks B4 submission and report endpoints (FR-04, FR-08, FR-09 Must; FR-10 Should).
- B3 covers assignment CRUD, resource limits, test-case lifecycle, stale-grade marking, and exclusion of soft-deleted cases from grading. Assignment creation now requires and atomically stores all six resource-limit fields. Test cases are soft-deleted whenever their assignment has any submission, including a running submission; assignment/test-case writes return 404 for non-owners, including an owner demoted to `STUDENT`.
- After the review fixes, `gofmt -l .` returned no files, `go vet ./...` passed, and `go test -race -count=1 -v ./...` passed with `scheduler/.env`; the suite had one intentional skip (`TestHelperProcess`). Coverage was measured through API integration tests by running `go test -coverpkg=./internal/api -cover ./internal/api -count=1`, then repeating with `./internal/auth` and `./internal/courses`; results were API 72.0%, auth 72.9%, and courses 76.7%.
- The demoted-owner test fails when the lecturer-role predicate is removed from either assignment or test-case ownership SQL; both mutations were restored and checked.
- Review-fix mutation checks also passed: removing create-time resource-limit invariant validation made the invalid-limit API tests fail; reverting test-case deletion to check existing results made the no-result and `RUNNING` tests fail; disabling the hidden-case filter exposed the hidden item and failed the student assertion. Each mutation was restored and verified with `git diff --exit-code`.
- B4 implementation is complete on `feature/scheduler-submissions`: enrolled students can upload and enqueue Python submissions; history, scoped detail, and reports are available; reports redact hidden-test data for students. The existing FR-14 rate limiter and `JobQueue.Enqueue` are used, and the upload contract is documented.
- B4 validation passed on current `origin/develop`: `gofmt -l .` returned no files, `go vet ./...` passed, and `go test -race -count=1 -v ./...` passed with `scheduler/.env` (one intentional skip, `TestHelperProcess`). Coverage measured with `go test -coverpkg=./... -coverprofile=/tmp/b4-coverage.out ./...` was 75.7% total.

## In progress / carried over

- B4 is awaiting separate code/documentation commits, push, and PR creation; do not merge before review.
- B4 reuses the existing settings-backed FR-14 limiter and the `JobQueue.Enqueue(ctx, queue.Job{SubmissionID, UserID})` contract. Multipart field names are documented as an API assumption.
- Confirm the UAT window of 27 Oct–9 Nov with Pak Peter; no confirmation has been recorded yet.

## Blockers and risks

- The B4 endpoints are not available on `develop` until the PR is reviewed and merged.
- R6: the UAT schedule confirmation is overdue. The 27 Oct–9 Nov window is still a proposal, not a confirmed schedule.
- Feature freeze is 26 Oct. Prioritize B4 and UAT-critical requirements over unstarted frontend work if the schedule slips.

## Next week

- Complete B4 PR handoff after the Go gates pass. Implement the separate YAML Must issue before declaring FR-03 complete.
- Confirm the UAT schedule and prepare role-based test accounts and assignment scenarios.
