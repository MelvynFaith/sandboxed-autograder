# Sprint 4 review and retrospective

**Dates:** 27 Oct – 9 Nov 2026 (weeks 11–12, the QA, security testing and UAT sprint)
**Sprint goal:** the QA, security testing and UAT of WBS 6 (SRS 10.2): the adversarial suite, the
load test with the rate-limit check, failure injection and reconciliation, the UAT with the
instructor, and the fix of the critical defects it finds; the deliverables are the UAT report and an
application with no open critical defect.

> Written on 13 Nov 2026 by Melvyn Faith, PM, from the repository history (merge dates are GitHub's,
> in UTC), the week 11 to 13 notes, the sprint 3 review and the CI record. No review meeting was
> recorded for this sprint. The retrospective is drawn from what the record shows, and the action
> items are proposals for the team to confirm. The sprint itself closed on 9 Nov; what happened after
> that is kept apart, so that the record shows what each period produced.

## Review

### Delivered

Five pull requests were merged in the sprint, all on 27 and 30 Oct; GitHub shows no commit, pull
request, review or merge between 31 Oct and 11 Nov. The QA evidence below was produced on 30 Oct
on a local stack with the real Harness and sandbox, not on the campus server.

| Item | Requirements | Owner |
|---|---|---|
| Confirmation dialogs trap keyboard focus ([#116](https://github.com/MelvynFaith/sandboxed-autograder/pull/116)) and the week 10 note and owned API docs are refreshed ([#106](https://github.com/MelvynFaith/sandboxed-autograder/pull/106)), merged 27 Oct | NFR-11 | Erlangga31 |
| The Nginx site that serves the dashboard and proxies the API, and a smoke test that uploads through the API ([#118](https://github.com/MelvynFaith/sandboxed-autograder/pull/118)), merged 30 Oct; the dashboard loaded through Nginx in a browser with no Content-Security-Policy violation | SRS 13.2, NFR-09, R10 | Melvyn Faith |
| Adversarial report and the API-side attack scripts ([#120](https://github.com/MelvynFaith/sandboxed-autograder/pull/120)), merged 30 Oct: the sandbox suite passes 64 of 64 on the normal build and on the ptrace fallback; 28 API attack checks pass (object-level access, cross-class access, role escalation, sessions, hidden test cases, a flood of 200 uploads); a scan of `/proc` during grading found no test case content | SRS 11.3, WBS 6.1, NFR-10, FR-03 | Melvyn Faith |
| Failure injection on the local stack, 30 Oct: a backend and sandbox killed while a submission was `RUNNING` came back, requeued it and graded it, with nothing left behind (reported in #120) | NFR-04, WBS 6.3 | Melvyn Faith |
| The UAT scenario of SRS 11.5 through the REST API on the local stack, 30 Oct: ten different programs, five test cases of which two hidden; all ten automatic scores equal the hand-made ones, the lecturer read every program and exported the CSV, no student saw hidden content. This is not the UAT with the instructor | FR-17, FR-20, SRS 11.5 | Melvyn Faith |
| A preliminary load test of 30 Oct on a development machine with four workers: 5 to 100 simultaneous accounts completed (100 accounts: average 2.3 s, p95 4.0 s), 199 of 200 uploads from one account were refused with a reason, and the fairness limit held (409 ms against 560 ms); written up in [#122](https://github.com/MelvynFaith/sandboxed-autograder/pull/122), merged 12 Nov | NFR-01a, NFR-01b, FR-13, FR-14, WBS 6.2 (preliminary) | Melvyn Faith |
| Changelog catch-up ([#119](https://github.com/MelvynFaith/sandboxed-autograder/pull/119)), merged 30 Oct; the sprint 3 review and the week 10 and 11 notes ([#121](https://github.com/MelvynFaith/sandboxed-autograder/pull/121)), opened 30 Oct and merged 12 Nov | SRS 8.2.1 | Melvyn Faith |

### Merged after the sprint closed (12–13 Nov, week 13)

Fourteen pull requests merged on 12 Nov, the day the open reviews were done. They are listed because
they finish work that belongs to this sprint's goal.

| Item | Requirements | Owner |
|---|---|---|
| The lecturer UAT flow and scenario ([#113](https://github.com/MelvynFaith/sandboxed-autograder/pull/113), opened 24 Oct), the NFR-11 audit ([#114](https://github.com/MelvynFaith/sandboxed-autograder/pull/114), opened 24 Oct), the 44 px touch targets ([#117](https://github.com/MelvynFaith/sandboxed-autograder/pull/117), opened 25 Oct), the user manual flow notes ([#137](https://github.com/MelvynFaith/sandboxed-autograder/pull/137)) | SRS 11.5, NFR-11, WBS 5.5, WBS 7.2 | Erlangga31 |
| The campus server upgraded to `develop` with the Harness reinstalled and the dashboard behind Nginx; the smoke test passes with 21 checks, also through Nginx and again after a reboot; the QA scripts on the campus server: 28 of 28 API attack checks, no test case content in 353 scans of `/proc`, and a backend killed while a submission was `RUNNING` came back after 1.5 s and graded it 10 of 10 with nothing left ([#123](https://github.com/MelvynFaith/sandboxed-autograder/pull/123), [#127](https://github.com/MelvynFaith/sandboxed-autograder/pull/127)) | SRS 13.2, NFR-04, NFR-09, SRS 11.3 | Melvyn Faith |
| Coverage gates in CI: the scheduler must keep 70 % (77.2 % measured), the grader 90 % (98 %), the dashboard 80 % statements and 75 % branches (86.4 % and 81.8 %) ([#124](https://github.com/MelvynFaith/sandboxed-autograder/pull/124)) | NFR-07 | Melvyn Faith |
| More denied syscalls and the other syscall ABIs covered by the sandbox tests, with an audit of the allowlist ([#128](https://github.com/MelvynFaith/sandboxed-autograder/pull/128)) | SRS 11.3 | Melvyn Faith |
| Two advisories closed: the standard library fixes of Go 1.27.4 ([#135](https://github.com/MelvynFaith/sandboxed-autograder/pull/135)) and the Vite dev-server advisories reported by `npm audit` ([#136](https://github.com/MelvynFaith/sandboxed-autograder/pull/136)) | NFR-07 | Melvyn Faith |
| The `critical` label of SRS 10.4 created (none carries it); the departed member removed from CODEOWNERS ([#126](https://github.com/MelvynFaith/sandboxed-autograder/pull/126)); the week 12 and 13 notes ([#125](https://github.com/MelvynFaith/sandboxed-autograder/pull/125)) | SRS 10.4, SRS 8.2.1 | Melvyn Faith |
| A browser test of the dashboard on 12 to 13 Nov found six bugs, all labelled `bug` ([#129](https://github.com/MelvynFaith/sandboxed-autograder/issues/129) to [#134](https://github.com/MelvynFaith/sandboxed-autograder/issues/134)); the fixes are proposed in [PR #139](https://github.com/MelvynFaith/sandboxed-autograder/pull/139), which is open with changes requested | NFR-06, NFR-11, FR-08, FR-17 | Melvyn Faith, Erlangga31 |

### Not delivered

| Item | Reason | Plan |
|---|---|---|
| The UAT with the instructor and its report (WBS 6.4, SRS 11.5) | The date was never confirmed (R6), and the QA window of 27 Oct to 9 Nov ended without it. The three numbers of SRS 11.5 (100 % agreement, 0 interventions, 0 open critical defects) are therefore not measured. The API-level run of 30 Oct is not a substitute | Hold the UAT on an isolated database once the program is finished; a run sheet and the server steps are prepared. The report follows it |
| The official load test and its report (WBS 6.2) | No run is recorded. SRS 13.1 puts it on staging, which does not exist, and the campus server is the only host with the real build | After the code freeze, on the final build, and not while the UAT runs; the place is a team decision |
| Fix of the critical defects found by the UAT (WBS 6.5) | There was no UAT, so none were found. The label that records them did not exist until 12 Nov | The six bugs of the browser test are in [PR #139](https://github.com/MelvynFaith/sandboxed-autograder/pull/139) |
| The sprint's work on the campus server in the sprint | The server ran the build of 19 Oct, before the submission API, until 12 Nov, so the server-side checks of SRS 11.3 and 11.4 could not run during the sprint | Done on 12 Nov (see above) |
| The manual keyboard checks of NFR-11 and the closing of [#112](https://github.com/MelvynFaith/sandboxed-autograder/issues/112) | The audit and touch targets merged on 12 Nov; the issue is still open | Confirm what is left and close it, or say what is missing |

### Indicators (SRS 10.4)

| Indicator | Value |
|---|---|
| WBS packages done | Not re-measured for this review. The user-story issues still open on 13 Nov are [#90](https://github.com/MelvynFaith/sandboxed-autograder/issues/90) (US-15, waits for the spreadsheet check of the CSV), [#94](https://github.com/MelvynFaith/sandboxed-autograder/issues/94) (US-19, waits for the official fairness run), [#111](https://github.com/MelvynFaith/sandboxed-autograder/issues/111) and [#112](https://github.com/MelvynFaith/sandboxed-autograder/issues/112) |
| Open critical defects | 0 issues carry the label on 13 Nov. The label did not exist before 12 Nov, so the figure could not be queried during the sprint. The six bugs of the browser test are labelled `bug`; whether #129 (a wrong grade) meets the critical definition of SRS 11.5 is still to be decided |
| CI success on `develop` | 7 of 7 runs passed between 27 Oct and 9 Nov |
| Test coverage | Go 77.2 %, grader 98 %, dashboard 86.4 % statements and 81.8 % branches, measured on 12 Nov in [#124](https://github.com/MelvynFaith/sandboxed-autograder/pull/124); the gates are 70 %, 90 % and 80 % / 75 % |

### Demo notes

- No demo was recorded and the instructor saw none. The checks above were run by the PM on a local
  stack on 30 Oct and on the campus server on 12 Nov; the campus server answered from outside the
  campus network (a phone on mobile data, 12 Nov, R10).
- The campus server is on plain HTTP: there is no DNS name yet, so R10 is reduced but not closed.

## Retrospective

### What went well

- The QA evidence is concrete and was reproduced where it counts: the API attack checks, the scans of
  `/proc` and the crash recovery of 30 Oct held again on the campus server on 12 Nov, and each report
  says what it did not test.
- Once the reviews were done, the backlog cleared in one day: 14 pull requests merged on 12 Nov, and
  the campus server was upgraded and smoke-tested, and a database restore test passed, the same day.
- New gates were added rather than only checks: coverage thresholds in CI, and two advisories (the Go
  standard library and the Vite dev server) were fixed on 12 Nov.

### What to improve

- The sprint's repository record is empty for twelve days (31 Oct to 11 Nov). Whatever was done
  outside the repository left no evidence for the report, and the QA window ended without the UAT.
- Pull requests waited. #113, #114, #117, #121 and #122 were open for 13 to 19 days; the NFR-11 work,
  the UAT scenario and the QA documents stayed out of `develop` the whole sprint.
- The deployment lagged the code: the server ran the build of 19 Oct until 12 Nov, so QA on the real
  host started after the sprint was over.
- The UAT date went unconfirmed through a fourth sprint (R6), and the QA window had no fallback.
- There is no staging host, so SRS 13.1 cannot be followed as written; the load test has no agreed
  place.
- The browser test of the dashboard came late and found six bugs in a day; a lecturer-level pass of
  the UI earlier would have found them inside the window.

### Action items

Proposed by the PM; to be confirmed with the team.

| Action | Owner | Due |
|---|---|---|
| Rework [#139](https://github.com/MelvynFaith/sandboxed-autograder/pull/139) for the two points of the review, then review and merge | Erlangga31, reviewers | 16 Nov |
| Redeploy the dashboard once after #139, run the smoke test and repeat the browser test | Melvyn Faith | After #139 |
| Hold the UAT on an isolated database, record the numbers of SRS 11.5 and the defects, and write the UAT report | Melvyn Faith with the instructor | Before the release; date not yet set |
| Run the official load test on the final build and write its report; decide where it runs and who runs it | Team | After the code freeze |
| Decide whether `Result.actual_output` is cleared after the retention period | Team | Before the BAST |
| Close or re-scope #9, #111 and #112 now that #113 and #114 are merged | Melvyn Faith | 16 Nov |
| Review pull requests within three days, or say when they will be reviewed | Reviewers | From now on |
