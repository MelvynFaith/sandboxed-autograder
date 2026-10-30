# Sprint 3 review and retrospective

**Dates:** 13 Oct – 26 Oct 2026 (weeks 9–10, ending at the feature freeze)
**Sprint goal:** the supporting features and refinement of WBS 5: submission history and detailed
reports, grade recap and CSV export, audit log and administration panel, regrade and cancel,
responsiveness and accessibility, bug fixing; and the feature freeze with every Must-have
requirement (SRS chapter 4) done.

> Written on 30 Oct 2026 by Melvyn Faith, PM, from the repository history (merge dates are GitHub's,
> in UTC), the week 09 and 10 notes, the GitHub Projects board and the CI record. No review meeting
> was recorded for this sprint. The retrospective is drawn from what the record shows, and the action
> items are proposals for the team to confirm.

## Review

### Delivered

Every functional requirement FR-01 to FR-20 has its backend on `develop` and a dashboard screen,
except where noted under "Not delivered". 60 pull requests were merged in the sprint; the rows group
them.

| Item | Requirements | Owner |
|---|---|---|
| Sandbox Executor merged, with its notification fix and the ptrace fallback ([#17](https://github.com/MelvynFaith/sandboxed-autograder/pull/17), [#23](https://github.com/MelvynFaith/sandboxed-autograder/pull/23), [#41](https://github.com/MelvynFaith/sandboxed-autograder/pull/41)); adversarial suite on three builds in CI ([#42](https://github.com/MelvynFaith/sandboxed-autograder/pull/42)) | FR-05, FR-06, FR-12, NFR-02, NFR-03, R12 | Melvyn Faith |
| Grading pipeline: queue, Harness runner, worker pool, rate limits, settings, start-up sweep, attempt cap, process wiring, `/healthz`, progress over WebSocket ([#25](https://github.com/MelvynFaith/sandboxed-autograder/pull/25), [#27](https://github.com/MelvynFaith/sandboxed-autograder/pull/27), [#29](https://github.com/MelvynFaith/sandboxed-autograder/pull/29), [#31](https://github.com/MelvynFaith/sandboxed-autograder/pull/31), [#36](https://github.com/MelvynFaith/sandboxed-autograder/pull/36) to [#39](https://github.com/MelvynFaith/sandboxed-autograder/pull/39), [#46](https://github.com/MelvynFaith/sandboxed-autograder/pull/46), [#47](https://github.com/MelvynFaith/sandboxed-autograder/pull/47), [#49](https://github.com/MelvynFaith/sandboxed-autograder/pull/49)) | FR-08, FR-13, FR-14, NFR-04, NFR-05, NFR-09 | Melvyn Faith |
| Grading Harness: skeleton, output comparison, verdict mapping, sandbox runner, weighted scoring, progress lines, README ([#26](https://github.com/MelvynFaith/sandboxed-autograder/pull/26), [#33](https://github.com/MelvynFaith/sandboxed-autograder/pull/33), [#44](https://github.com/MelvynFaith/sandboxed-autograder/pull/44), [#45](https://github.com/MelvynFaith/sandboxed-autograder/pull/45), [#48](https://github.com/MelvynFaith/sandboxed-autograder/pull/48), [#65](https://github.com/MelvynFaith/sandboxed-autograder/pull/65), [#67](https://github.com/MelvynFaith/sandboxed-autograder/pull/67)) | FR-07, FR-10, FR-08 | Robertus Gabeler |
| Authentication, Course and enrollment, assignment, resource limit and test case APIs, and the submission API ([#35](https://github.com/MelvynFaith/sandboxed-autograder/pull/35), [#51](https://github.com/MelvynFaith/sandboxed-autograder/pull/51), [#59](https://github.com/MelvynFaith/sandboxed-autograder/pull/59), [#63](https://github.com/MelvynFaith/sandboxed-autograder/pull/63)) | FR-01, FR-02, FR-03, FR-04, FR-06, FR-09, FR-10, FR-15, FR-18, FR-19, NFR-10, NFR-12 | Benedictus Erlangga |
| Regrade and cancel, code view, grade recap and CSV, administrator endpoints, test case YAML, dashboard fields ([#97](https://github.com/MelvynFaith/sandboxed-autograder/pull/97), [#74](https://github.com/MelvynFaith/sandboxed-autograder/pull/74), [#69](https://github.com/MelvynFaith/sandboxed-autograder/pull/69), [#73](https://github.com/MelvynFaith/sandboxed-autograder/pull/73), [#87](https://github.com/MelvynFaith/sandboxed-autograder/pull/87), [#68](https://github.com/MelvynFaith/sandboxed-autograder/pull/68)) | FR-03, FR-09 to FR-14, FR-16, FR-17, FR-20 | Melvyn Faith |
| Dashboard: registration and profile, status handling, Course and enrollment UI, lecturer assignment and test case screens, YAML import/export, polling fix, dialog focus trap ([#16](https://github.com/MelvynFaith/sandboxed-autograder/pull/16), [#19](https://github.com/MelvynFaith/sandboxed-autograder/pull/19), [#22](https://github.com/MelvynFaith/sandboxed-autograder/pull/22), [#32](https://github.com/MelvynFaith/sandboxed-autograder/pull/32), [#54](https://github.com/MelvynFaith/sandboxed-autograder/pull/54), [#99](https://github.com/MelvynFaith/sandboxed-autograder/pull/99), [#100](https://github.com/MelvynFaith/sandboxed-autograder/pull/100), [#104](https://github.com/MelvynFaith/sandboxed-autograder/pull/104), [#110](https://github.com/MelvynFaith/sandboxed-autograder/pull/110), [#116](https://github.com/MelvynFaith/sandboxed-autograder/pull/116)) | FR-01, FR-02, FR-03, FR-06, FR-15, FR-18, NFR-11 (partly) | Benedictus Erlangga |
| Dashboard: history and reports from the real API, cancel, code view, recap with CSV and regrade, administration page, resource limits on assignment creation ([#70](https://github.com/MelvynFaith/sandboxed-autograder/pull/70), [#71](https://github.com/MelvynFaith/sandboxed-autograder/pull/71), [#72](https://github.com/MelvynFaith/sandboxed-autograder/pull/72), [#98](https://github.com/MelvynFaith/sandboxed-autograder/pull/98)) | FR-09 to FR-12, FR-14, FR-16, FR-17, FR-19, FR-20 | Melvyn Faith |
| Operations on the campus server: install steps, smoke test, daily backup, expiry of old submission files, AppArmor profile and unit, k6 load test scripts ([#24](https://github.com/MelvynFaith/sandboxed-autograder/pull/24), [#50](https://github.com/MelvynFaith/sandboxed-autograder/pull/50), [#55](https://github.com/MelvynFaith/sandboxed-autograder/pull/55) to [#57](https://github.com/MelvynFaith/sandboxed-autograder/pull/57), [#62](https://github.com/MelvynFaith/sandboxed-autograder/pull/62), [#64](https://github.com/MelvynFaith/sandboxed-autograder/pull/64)) | NFR-08, NFR-09, NFR-13, R9, R10 (partly) | Melvyn Faith |
| OpenAPI document of every registered route, with a test that fails when one is missing ([#101](https://github.com/MelvynFaith/sandboxed-autograder/pull/101), [#108](https://github.com/MelvynFaith/sandboxed-autograder/pull/108), [#115](https://github.com/MelvynFaith/sandboxed-autograder/pull/115)) | WBS 2.2 | Benedictus Erlangga |
| Weekly notes, the backfilled notes and the sprint 1 and 2 reviews ([#20](https://github.com/MelvynFaith/sandboxed-autograder/pull/20), [#40](https://github.com/MelvynFaith/sandboxed-autograder/pull/40), [#53](https://github.com/MelvynFaith/sandboxed-autograder/pull/53), [#66](https://github.com/MelvynFaith/sandboxed-autograder/pull/66), [#106](https://github.com/MelvynFaith/sandboxed-autograder/pull/106)) | SRS 10.4 | Erlangga, Gabeler, Faith |

### Not delivered

| Item | Reason | Plan |
|---|---|---|
| Responsiveness and accessibility (WBS 5.5, NFR-11) | The audit report ([#114](https://github.com/MelvynFaith/sandboxed-autograder/pull/114)) and the 44 px touch targets ([#117](https://github.com/MelvynFaith/sandboxed-autograder/pull/117)) are approved but not merged; the keyboard checks are only partly done | Merge both; the manual keyboard checks before [#112](https://github.com/MelvynFaith/sandboxed-autograder/issues/112) is closed |
| The lecturer UAT flow and scenario ([#113](https://github.com/MelvynFaith/sandboxed-autograder/pull/113)) | Two reviews asked for changes (the scenario has to match SRS 11.5; the ten programs have to differ); the isolated-database run is blocked on a database role | Rework by the author; the API-side UAT run exists meanwhile (see Demo notes) |
| Bug fixing and query optimization (WBS 5.6) | No performance measurement was made, so there is nothing to optimize against | The load test (WBS 6.2) gives the first numbers |
| The sprint's work on the campus server | No deployment after 19 Oct is recorded; the server runs the build from before the submission API | Deploy once [#118](https://github.com/MelvynFaith/sandboxed-autograder/pull/118) (Nginx and the dashboard) is merged |
| Fairness test and spreadsheet check of the CSV ([#94](https://github.com/MelvynFaith/sandboxed-autograder/issues/94), [#90](https://github.com/MelvynFaith/sandboxed-autograder/issues/90)) | Not run | Load test; open the CSV in Excel and Google Sheets |

### Indicators (SRS 10.4)

| Indicator | Value on 30 Oct |
|---|---|
| WBS packages done | 19 of 21 user-story cards Done on 30 Oct, after the board was brought up to date that day. Until then it showed 6 Done, 10 Review, 4 In Progress and 1 To Do, because it had not followed the merges. The two cards left are US-15 (grade recap, awaiting the check of the CSV in a spreadsheet, issue #90) and US-19 (fairness under load, issue #94, To Do). The cards moved to Done are the ones whose issues were closed on 30 Oct with the evidence for each criterion |
| Open critical defects | 0 recorded; the label `critical` does not exist yet, so the figure cannot be queried. It is to be created |
| CI success on `develop` | 62 of 62 runs passed between 13 and 26 Oct |
| Test coverage | Go: 77.2% of statements across the packages, measured on 30 Oct at `62908c7` with `go test -coverpkg=./...` (the target is 70%). Python and the frontend are not measured |

### Demo notes

- No demo was recorded. On 30 Oct the UAT scenario of SRS 11.5 was run through the REST API on a local
  stack with the real Harness and sandbox: a lecturer, ten students, five test cases of which two
  hidden, ten different programs. All ten automatic scores equalled the hand-made ones (100, 100, 100,
  40, 0, 0, 0, 0, 0, 20), the lecturer read every program's code and exported the CSV, and no student
  saw hidden content. This is not the UAT with the instructor.

## Retrospective

### What went well

- The backend caught up: from no scheduler code on 12 Oct to a working pipeline with every Must-have
  endpoint on `develop` on 22 Oct.
- A batch of ten pull requests opened on 19 Oct was reviewed and merged in one to three days, and CI
  stayed green throughout (62 of 62).
- Reviews found real defects before merge: the upload status code and the hidden-test leak in
  [#63](https://github.com/MelvynFaith/sandboxed-autograder/pull/63), the missing lecturer email for
  administrators in [#99](https://github.com/MelvynFaith/sandboxed-autograder/pull/99).

### What to improve

- QA started late. The load test scripts exist but have never run, no UAT has been held, and there
  is no staging host, so the checks of SRS 13.1 had nowhere to run.
- The board and the issues did not follow the work: most cards sat in Review or In Progress after
  their pull requests merged.
- The UAT date has gone unconfirmed through three sprints (R6).
- The Harness and QA packages (WBS 4.1, 6.2) have had no merge since 19 Oct, and the load test and
  the injection test (WBS 6.2, 6.3) had no owner who started them. The team should say now who can
  take each, and by when.

### Action items

Proposed by the PM; to be confirmed with the team.

| Action | Owner | Due |
|---|---|---|
| Confirm the UAT date with the instructor (R6) | Melvyn Faith (PM) | 31 Oct |
| Create the `critical` label (the board was brought up to date on 30 Oct) | Melvyn Faith | 31 Oct |
| Merge #117, rebase and merge #114; the manual keyboard checks of NFR-11 | Benedictus Erlangga | 3 Nov |
| Rework #113 (varied programs, hidden case, full scenario) | Benedictus Erlangga | 3 Nov |
| Deploy to the campus server (after #118) and run the smoke test and the QA scripts there | Melvyn Faith | 3 Nov |
| Run the load test and write its report (WBS 6.2); open the CSV in a spreadsheet | Robertus Gabeler | 6 Nov |
| Decide where the official load test runs, since 13.1 names staging and there is none | Team | 3 Nov |
| Decide whether `Result.actual_output` is cleared after the retention period | Team | 6 Nov |
