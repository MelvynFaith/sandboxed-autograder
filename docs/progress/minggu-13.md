# Week 13 progress (10 Nov - 16 Nov 2026)

**Phase / sprint:** Deployment and handover (WBS 7)

> State on Thursday 12 Nov 2026, written by Melvyn Faith from the repository, GitHub and the run of
> the upgrade on the campus server. It is to be updated before the week closes on 16 Nov. Dates are
> GitHub's, in UTC; times of the server's timers are in WIB.

## Done

- **Owner: Melvyn Faith.** The campus server runs `develop` at `eeaa8eb` since 12 Nov. A database dump
  was taken first, the Grading Harness was reinstalled, and the backend was rebuilt; the previous
  binary is kept for a rollback. The schema is still version 4 (no migration since 19 Oct). At
  start-up the backend reported the schema up to date, the delegated cgroup ready, the four workers
  started and the sweep of leftovers done, and `/healthz` answered with four of four workers.
- **Owner: Melvyn Faith.** The smoke test (`infra/smoke-test.sh`) passes on the campus server with 21
  checks: three students register, log in and upload through `POST /api/v1/submissions`, the second
  upload of a student is refused with `429`, the scores are 100, 0 and 0, the denied `socket()` call
  is audited with its number, memory comes from the cgroup, and no sandbox cgroup or `ERROR` line is
  left.
- **Owner: Melvyn Faith.** Nginx (Ubuntu's 1.24) is installed on the campus server and serves the
  dashboard build and proxies `/api/`, `/ws/` and `/healthz` from `infra/nginx/autograder.conf`
  (step 11 of `infra/README.md`). The start page and a client route answer `200`, a missing asset
  `404`, and the smoke test passes through Nginx as well. The site is plain HTTP.
- **Owner: Melvyn Faith.** After a reboot for a pending kernel update, PostgreSQL, the backend and
  Nginx were all running again without any command, and the smoke test through Nginx passed a second
  time.
- **Owner: Melvyn Faith.** The daily jobs of the campus server are confirmed: the timers of the
  database backup (02:30 WIB) and of the expiry of old submission files (03:30 WIB) had both last run
  in the night of 11 to 12 Nov, and the dumps of 8 to 11 Nov are there. This closes the check that was
  open since 20 Oct.
- **Owner: Melvyn Faith.** The QA scripts ran on the campus server: 28 of 28 API attack checks, no
  test case content in 353 scans of `/proc` as the service user, and a backend killed while a
  submission was `RUNNING` came back after 1.5 s and graded it 10 of 10 with nothing left. Report in
  [PR #123](https://github.com/MelvynFaith/sandboxed-autograder/pull/123), which also fixes the
  documented kill command of the crash test.
- **Owner: Melvyn Faith.** Coverage gates for CI (NFR-07), in
  [PR #124](https://github.com/MelvynFaith/sandboxed-autograder/pull/124): the scheduler must keep 70
  % statement coverage (77.2 % measured), the grader 90 % (98 %), and the dashboard thresholds of 80
  % statements and 75 % branches (86.4 % and 81.8 %). CI on that pull request passed with the gates
  on.

## In progress / carried over

- **Owner: Erlangga31.** [PR #117](https://github.com/MelvynFaith/sandboxed-autograder/pull/117) (44
  px touch targets) is approved; [PR #114](https://github.com/MelvynFaith/sandboxed-autograder/pull/114)
  (the NFR-11 audit) is approved but conflicts with `develop`;
  [PR #113](https://github.com/MelvynFaith/sandboxed-autograder/pull/113) (the lecturer UAT flow) has
  had changes requested since 27 Oct.
- **Owner: Robertus Gabeler.** The official load test and its report (WBS 6.2) have no recorded run.
  The campus server is now ready for it, on the build that will be released.
- **Owner: Melvyn Faith.** [PR #121](https://github.com/MelvynFaith/sandboxed-autograder/pull/121),
  [#122](https://github.com/MelvynFaith/sandboxed-autograder/pull/122),
  [#123](https://github.com/MelvynFaith/sandboxed-autograder/pull/123) and
  [#124](https://github.com/MelvynFaith/sandboxed-autograder/pull/124) are open for review; this note
  and the week 12 note are in the pull request that adds them.
- **Owner: Melvyn Faith.** The release of `develop` to `main` with a tag (SRS 13.3), the user manual
  (WBS 7.2) and the BAST package (SRS 13.5).

## Blockers and risks

- **R6, owner: Melvyn Faith.** The UAT with the instructor has no confirmed date, and the QA window
  ended on 9 Nov. The deployment window ends on 23 Nov.
- **R10.** Nginx is up, but the proof that the dashboard opens from outside the campus network (from
  a phone on mobile data) is not recorded yet, and there is no DNS name for HTTPS.
- **Owner: project team.** SRS 13.1 puts the load test on staging, and there is no staging host. The
  campus server is the only host with the real build; the team has to decide whether the official run
  happens there, and when, so that it does not disturb the UAT.
- **Owner: Melvyn Faith.** The `critical` label of SRS 10.4 still does not exist, so the defect
  indicator cannot be queried.
- **Owner: project team.** Seven pull requests wait for a review or for rework, besides the one
  that adds this note; until they merge, `develop` does not contain the QA reports, the coverage gates
  or the NFR-11 work.

## Next week

- Merge the open pull requests and rework #113 and #114.
- Run the official load test on the campus server and write its report; close
  [issue #94](https://github.com/MelvynFaith/sandboxed-autograder/issues/94) with its result, and
  [issue #90](https://github.com/MelvynFaith/sandboxed-autograder/issues/90) after the CSV is opened
  in a spreadsheet.
- Confirm the UAT date with the instructor and hold the UAT; record the defects it finds.
- Release `develop` to `main` with a tag, and hand over: user manual, BAST package, final SRS status.
