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
  - [PR #37](https://github.com/MelvynFaith/sandboxed-autograder/pull/37), [PR #38](https://github.com/MelvynFaith/sandboxed-autograder/pull/38), and [PR #39](https://github.com/MelvynFaith/sandboxed-autograder/pull/39) — scheduler settings, startup sweep, and lost-submission attempt limit. Owner: Melvyn Faith.
- On 18 Oct, the Python Grading Harness was completed on `develop`. Owner: whosblub.
  - [PR #44](https://github.com/MelvynFaith/sandboxed-autograder/pull/44) — verdict mapping (SRS 3.2.1.7).
  - [PR #45](https://github.com/MelvynFaith/sandboxed-autograder/pull/45) — `sandbox-exec` runner (Python side; PR #29 is the Go side).
  - [PR #48](https://github.com/MelvynFaith/sandboxed-autograder/pull/48) — weighted scoring, report, and `python -m harness` entry point. The grader suite has 97 tests and `ruff check` is clean.
- Manual integration test of the Harness against the real `sandbox-exec` (WSL2, kernel 6.6.87, Python 3.12; the test scripts are not in the repository yet). A 3-case sample gives `total_score` 5 of `max_score` 6. Twelve adversarial programs all ended with the expected verdict:
  - infinite loop and sleep stall: `TIMEOUT`; memory bomb: `MEMORY_LIMIT_EXCEEDED` at 64 MB; output bomb: `FAILED` with `is_truncated`.
  - fork bomb: `RUNTIME_ERROR` (`BlockingIOError`), so `pids_limit_hit` needs no new verdict.
  - disk bomb: `RUNTIME_ERROR`, not `MEMORY_LIMIT_EXCEEDED`.
  - `socket()`: `SECURITY_VIOLATION` (syscall 41); `ptrace`: `SECURITY_VIOLATION` (syscall 101); `/etc/passwd`: `FileNotFoundError`.
  - Failure paths (missing `sandbox-exec`, wrong cgroup root, missing environment variables) exit 1 with empty stdout, so the worker marks the submission `FAILED`.
- B0 local backend preparation: Go 1.27.2 and PostgreSQL 18.6 are available, initial migrations were applied, and `gofmt`, `go vet`, and `go test -race` passed. Local development credentials remain in the ignored `scheduler/.env`.
- The CRUD/pipeline boundary is documented in the scheduler layout: scheduler owns HTTP/API orchestration and is the only PostgreSQL writer; grader and sandbox own submitted-code execution. The `JobQueue` boundary carries `SubmissionID` and `UserID`.
- Added on 19 Oct by Melvyn Faith (PM from that date): the rest of the week, from the repository history.
- Grading pipeline and its operation, merged 17–19 Oct. Owner: Melvyn Faith.
  - [PR #41](https://github.com/MelvynFaith/sandboxed-autograder/pull/41): `ptrace` fallback for kernels without seccomp user notification (R12). [PR #42](https://github.com/MelvynFaith/sandboxed-autograder/pull/42): CI runs the adversarial suite on the normal, `ptrace` and ASan builds.
  - [PR #46](https://github.com/MelvynFaith/sandboxed-autograder/pull/46): the `autograder` process runs the migrations, the delegated cgroup and the worker pool next to the API. [PR #47](https://github.com/MelvynFaith/sandboxed-autograder/pull/47): `/healthz` (NFR-09). [PR #49](https://github.com/MelvynFaith/sandboxed-autograder/pull/49): grading progress over WebSocket (FR-08).
  - [PR #50](https://github.com/MelvynFaith/sandboxed-autograder/pull/50): backend install steps and `infra/smoke-test.sh`. [PR #52](https://github.com/MelvynFaith/sandboxed-autograder/pull/52): `autograder seed-admin` for the first administrator (FR-19).
  - [PR #55](https://github.com/MelvynFaith/sandboxed-autograder/pull/55) and [PR #56](https://github.com/MelvynFaith/sandboxed-autograder/pull/56): daily database backup with 14-day retention, and fixes to the install steps found during the deploy. [PR #62](https://github.com/MelvynFaith/sandboxed-autograder/pull/62): stored submission files are deleted a year after their assignment closes (NFR-13).
  - [PR #57](https://github.com/MelvynFaith/sandboxed-autograder/pull/57): k6 load tests for every scenario of SRS 11.4, with seeded student accounts. They cannot run end to end until the upload endpoint exists.
- Merged on 19 Oct. Owner: Benedictus Erlangga.
  - [PR #51](https://github.com/MelvynFaith/sandboxed-autograder/pull/51): course and enrollment API (FR-18).
  - [PR #54](https://github.com/MelvynFaith/sandboxed-autograder/pull/54): register, profile and logout pages.
  - [PR #59](https://github.com/MelvynFaith/sandboxed-autograder/pull/59): assignment, resource-limit and test-case API (B3).
- Campus server, 17–19 Oct. Owner: Melvyn Faith.
  - SRS table 6.1 filled in with measured values. R9 applies (AppArmor blocks unprivileged user namespaces) and is solved by a profile bound to the installed `sandbox-exec`.
  - The backend runs there as the systemd service `autograder`. The adversarial suite passed 64/64 as a system service, the smoke test passed 13/13, and `/healthz` answered `503` with PostgreSQL stopped and `200` once it was back, without a backend restart.
  - The first administrator was created. The daily backup and file-expiry timers are enabled; a backup was restored into a scratch database. The schema is at version 4.
  - The backend still listens on the loopback address only: there is no Nginx, TLS or DNS name yet (R10).
- SRS 1.4 (17 Oct) recorded the server verification, and SRS 1.5 (19 Oct) the deployment and the data model changes.

## In progress / carried over

- B1 auth/session API (WBS 3.4; FR-01, FR-15, FR-19) was merged to `develop` through [PR #35](https://github.com/MelvynFaith/sandboxed-autograder/pull/35), squash commit `2e4cb82`. All five CI checks passed before merge. Coverage with `-coverpkg=./...`: `internal/api` 70.7%, `internal/auth` 76.4%, total 76.7%.
- Implement the FR-11 grade-summary/CSV frontend UI; confirm its backend endpoint contract and backend owner.
- B2 Course and enrollment (FR-18) depends on the auth middleware/helpers now merged in B1. Start it from updated `develop`.
- Harness QA, second round: self-signal (`kill` on itself), `multiprocessing`, `/proc` reads, writes outside `/work`, and repeated runs to check for leftover cgroups and run directories. Then turn the scenarios into automated integration tests that are skipped when `sandbox-exec` is absent. Owner: whosblub.
- Update `grader/README.md` (still says "not started") and confirm whether the YAML test-case import/export is still in scope before the freeze. Owner: whosblub.
- [PR #63](https://github.com/MelvynFaith/sandboxed-autograder/pull/63), the submission API (B4: upload, history, detail, report), was opened on 19 Oct and is in review. Owner: Benedictus Erlangga.
- From 19 Oct, PM work and project documents move from Benedictus Erlangga to Melvyn Faith: these notes, sprint reviews, UAT coordination, the user manual and the handover. Benedictus keeps the documentation of his own backend and frontend work.

## Blockers and risks

- R6: the final UAT schedule confirmation deadline (12 Oct, end of Week 8) has passed. The schedule is **not confirmed**, and there is no evidence that a message to the instructor has been sent. The proposed UAT window is 27 Oct–9 Nov. The SRS fallback is students from another class participating under instructor supervision; the PM, Benedictus Erlangga, decides whether to use it.
- R4 and R7: Evan has left; he was the planned backup for the sandbox module and a reviewer for Module C. This reduces sandbox backup and review capacity; replacement coverage has not been confirmed.
- Feature freeze is 26 Oct and UAT starts 27 Oct. The team still needs to decide which Should/Could requirements can be deferred and confirm SRS ownership and API contracts.
- Harness scoring is all or nothing per test case (full weight for `PASSED`, otherwise 0). The SRS does not mention partial credit; the team needs to confirm this.

## Next week

- Start B2 (FR-18) from updated `develop`.
- Confirm replacement sandbox backup and Module C reviewer coverage (R4, R7).
- Have the PM decide whether to use the SRS UAT fallback (R6); document a confirmed schedule only after confirmation.
- Agree Should/Could scope cuts before the 26 Oct feature freeze, and confirm the backend owner/API contract for FR-11.
