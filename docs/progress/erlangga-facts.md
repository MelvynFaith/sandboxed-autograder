# Erlangga — verified work facts

This file records work by Erlangga31 only. It is a source for weekly progress notes and the user
manual, not a complete project status report. SRS text is maintained privately and was not included
in this repository.

## PR #63 — submission API

- **Status:** merged into `develop` on 2026-10-19 as squash commit `544173b`.
- **Requirements:** FR-04, FR-08, FR-09, FR-10, FR-14, and NFR-10. Issue #61 explicitly marks
  FR-10 as Should; it does not state priorities for the other requirements.
- **Endpoints:** `POST /api/v1/submissions`; `GET /api/v1/submissions` with pagination and
  `assignment_id`; `GET /api/v1/submissions/{id}`; `GET /api/v1/submissions/{id}/report`.
- **Verified behavior:** student upload uses multipart `assignment_id` and one `file` part; `.py`
  only, 256 KiB maximum; server-generated stored filename; accepted requests return 202 with status
  `QUEUED`; late-window uploads set `is_late` without a score penalty; after `late_until_at` returns
  409; rate-limit rejection is 429. Other students' and Courses' submission objects return 404.
  Hidden-test report content is visible to admins and current Course owners; other viewers receive
  only verdict, score, execution time, and memory fields for hidden results.
- **Validation recorded in the PR:** `gofmt`, `go vet`, and `go test -race` passed; one intentional
  `TestHelperProcess` skip; total coverage 75.7%. The PR also recorded mutation checks for the
  202 response and hidden-result authorization.
- **Reference:** [PR #63](https://github.com/MelvynFaith/sandboxed-autograder/pull/63).

## PR #99 — Course and enrollment UI

- **Status:** open; review requested from MelvynFaith. Rebased onto `origin/develop`; all required CI
  checks passed, but approval is still required.
- **Requirements:** FR-18, NFR-10, NFR-12.
- **Pages:** `/courses` for lecturers/admins; Course-scoped `/assignments` for students; lecturer and
  admin Course assignment access.
- **Verified behavior:** Course creation, enrollment add/remove, student Course list and grouped
  assignment view. The frontend consumes the Course/enrollment API contract already in `develop`.
- **Local validation:** lint and build passed. The pre-rebase run was 19 files / 100 tests. After
  rebase, one full local run had a History test fail while the page was still loading; the same test
  passed in isolation and the frontend CI job subsequently passed.
- **Responsive check:** Course and Assignment routes measured at 360, 768, and 1280 CSS pixels; no
  horizontal overflow. Form controls were label-associated, and keyboard focus was visible.
- **Reference:** [PR #99](https://github.com/MelvynFaith/sandboxed-autograder/pull/99).

## PR #100 — test-case YAML UI

- **Status:** open; review requested from MelvynFaith; all required CI checks passed, but approval is
  still required.
- **Requirements:** FR-03.
- **Page:** YAML import/export controls on `/lecturer/assignments`.
- **Verified behavior:** export downloads the server-provided filename; import sends the selected
  YAML document through the authenticated API client, enforces the 1 MiB client-side limit, displays
  backend validation messages and created/updated counts, then refreshes test cases and resource
  limits. The UI follows the backend contract in PR #87, which is not yet merged.
- **Local validation:** lint and production build passed; full frontend suite passed (18 files, 99
  tests). Browser check of the lecturer assignment management page at 360, 768, and 1280 CSS pixels
  found no horizontal overflow; form fields had labels and keyboard focus was visible.
- **Reference:** [PR #100](https://github.com/MelvynFaith/sandboxed-autograder/pull/100).

## Current gaps and dependencies

- Backend PRs #69 (grade recap/CSV), #74 (submission code), #73 (admin endpoints), and #87 (YAML
  import/export) are authored by MelvynFaith and remain outside Erlangga31's branch ownership.
- Frontend PRs #71 (admin panel), #72 (history/report/cancel integration), and #98 (code view,
  recap/CSV, regrade) are also authored by MelvynFaith. Do not duplicate their changes.
- The NFR-11 browser checks recorded above covered the current owned Course and lecturer assignment
  surfaces. Routes introduced only by teammate PRs were not included in that verification.
- `scheduler/README.md` already documents the submission endpoints and behavior from PR #63; no
  further scheduler README change was needed for that merged feature.
- The OpenAPI file added in this documentation change covers only the submission endpoints from
  PR #63. YAML, code-view, recap, admin, and cancel/regrade API documentation remains dependent on
  the respective backend changes being integrated and on later documentation work.
