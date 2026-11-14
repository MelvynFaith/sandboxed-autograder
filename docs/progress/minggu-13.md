# Week 13 progress (10 Nov - 16 Nov 2026)

**Phase / sprint:** Deployment and handover (WBS 7)

> State on Friday 13 Nov 2026, written by Melvyn Faith from the repository, GitHub and the runs on the
> campus server. It replaces the Thursday version and is to be updated once more if anything changes
> before the week closes on 16 Nov. Dates are GitHub's, in UTC, so the evening of 12 Nov in WIB falls on
> 13 Nov here; times of the server's timers are in WIB.

## Done

- **Owner: Melvyn Faith.** The campus server runs `develop` since 12 Nov. The first upgrade (from
  `eeaa8eb`) took a database dump, reinstalled the Grading Harness and rebuilt the backend, and kept the
  previous binary for a rollback. The schema is still version 4 (no migration since 19 Oct). At
  start-up the backend reported the schema up to date, the delegated cgroup ready, the four workers
  started and the sweep of leftovers done, and `/healthz` answered with four of four workers. Later that
  day the backend was rebuilt once more, from `develop` at `0241b01`, with Go 1.27.4 (see the
  dependency advisories below), after another dump; the same start-up lines and the smoke test followed.
- **Owner: Melvyn Faith.** The smoke test (`infra/smoke-test.sh`) passes on the campus server with 21
  checks, after both rebuilds: three students register, log in and upload through `POST
  /api/v1/submissions`, the second upload of a student is refused with `429`, the scores are 100, 0 and
  0, the denied `socket()` call is audited with its number, memory comes from the cgroup, and no sandbox
  cgroup or `ERROR` line is left.
- **Owner: Melvyn Faith.** Nginx (Ubuntu's 1.24) is installed on the campus server and serves the
  dashboard build and proxies `/api/`, `/ws/` and `/healthz` from `infra/nginx/autograder.conf`
  (step 11 of `infra/README.md`). The start page and a client route answer `200`, a missing asset
  `404`, and the smoke test passes through Nginx as well. The site is plain HTTP. The dashboard build
  that is served is the one of 12 Nov, from before the pull requests listed below were merged.
- **Owner: Melvyn Faith.** The firewall of the campus server (`ufw`) was inactive although the server
  has a public address on its own interface (SRS 6.1). It is active since 12 Nov, with incoming traffic
  denied by default and ports 22 and 80 allowed (IPv4 and IPv6). It needs a line in the SRS at the next
  revision, and port 443 when HTTPS comes.
- **Owner: Melvyn Faith. R10:** on 12 Nov, after the firewall was on, from a phone on mobile data with
  Wi-Fi off, that is from outside the campus network, the login page of the dashboard opened over HTTP on
  the server's public address, and `/healthz` answered `"status":"ok"`. The address is not recorded in
  the repository.
- **Owner: Melvyn Faith.** The `critical` label of SRS 10.4 was created on 12 Nov, so the indicator
  "open critical defects" can now be queried; no issue carries it yet.
- **Owner: Melvyn Faith.** After a reboot for a pending kernel update, PostgreSQL, the backend and
  Nginx were all running again without any command, and the smoke test through Nginx passed a second
  time.
- **Owner: Melvyn Faith.** The daily jobs of the campus server are confirmed: the timers of the
  database backup (02:30 WIB) and of the expiry of old submission files (03:30 WIB) had both last run
  in the night of 11 to 12 Nov, and the dumps of 8 to 11 Nov are there. This closes the check that was
  open since 20 Oct.
- **Owner: Melvyn Faith. Recovery drills (SRS 13.3 and 13.4), merged in
  [PR #127](https://github.com/MelvynFaith/sandboxed-autograder/pull/127):** the backup of that day was
  restored into a scratch database (`pg_restore` exited 0, schema version 4 and not dirty, the same
  single user as the live database), and the rollback was rehearsed: with the previous binary installed
  the service came back `active` with four of four workers, and with the new binary installed again the
  smoke test passed. `infra/README.md` now has an "Upgrading and rolling back" section with both
  procedures and what the drills did not cover (a rollback across a migration, and the pipeline of the
  previous binary).
- **Owner: Melvyn Faith. QA on the campus server, merged in
  [PR #123](https://github.com/MelvynFaith/sandboxed-autograder/pull/123):** 28 of 28 API attack
  checks, no test case content in 353 scans of `/proc` as the service user, and a backend killed while a
  submission was `RUNNING` came back after 1.5 s and graded it 10 of 10 with nothing left. The pull
  request also fixes the documented kill command of the crash test.
- **Owner: Melvyn Faith. Audit of the syscall allowlist, merged in
  [PR #128](https://github.com/MelvynFaith/sandboxed-autograder/pull/128):** 77 system calls that are not
  on the allowlist were each issued from a Python program in the real sandbox, and all 77 were reported
  as `SECURITY_VIOLATION` with the call's number. The other call ABIs (the x32 bit and `int 0x80`) are
  killed and reported too. Two tests were added to the suite (24 of the 77 calls, and the ABIs); with a
  deliberate change of the filter each fails, and the suite passes 66 of 66 on the normal build and on
  the ptrace fallback build. The report lists what the allowlist leaves reachable on purpose and three
  candidates to narrow (`memfd_create`, the socket options, `uname`/`sysinfo`); the allowlist was not
  changed, because each candidate must first be tried against the UAT programs.
- **Owner: Melvyn Faith. Coverage gates for CI (NFR-07), merged in
  [PR #124](https://github.com/MelvynFaith/sandboxed-autograder/pull/124):** the scheduler must keep 70
  % statement coverage (77.2 % measured), the grader 90 % (98 %), and the dashboard thresholds of 80
  % statements and 75 % branches (86.4 % and 81.8 %).
- **Owner: Melvyn Faith. Dependency advisories:** `npm audit` reported two high advisories in the Vite
  dev server of the dashboard; the lockfile was moved to Vite 8.3.8 within the allowed range
  ([PR #136](https://github.com/MelvynFaith/sandboxed-autograder/pull/136)) and it now reports none.
  `govulncheck` reported two advisories in the standard library of Go 1.27.2 that the backend was built
  with (`net/http` and `crypto/x509`); `scheduler/go.mod` now requires Go 1.27.4
  ([PR #135](https://github.com/MelvynFaith/sandboxed-autograder/pull/135)), the campus server's backend
  was rebuilt with it, and `govulncheck` on the installed binary and on the source reports that no
  vulnerability affects the code. One advisory remains in a required module (the `openpgp` package of
  `golang.org/x/crypto`); the backend does not import that package and the advisory has no tagged fix.
- **Owner: Melvyn Faith. Browser test on the real backend:** a local stack (backend, real Harness and
  sandbox, Nginx) was driven through the dashboard as a lecturer and as a student: login, upload,
  progress, history, report (hidden cases show no input or output), code view, the rate limit message,
  assignment and test case creation and editing, and regrade. Nothing crashed, and the console showed no
  application error or CSP violation. Six defects were filed as issues
  [#129](https://github.com/MelvynFaith/sandboxed-autograder/issues/129) to
  [#134](https://github.com/MelvynFaith/sandboxed-autograder/issues/134): the default `EXACT` match
  fails a correct program when the expected output has no trailing newline and gives no hint; form
  messages appear off screen; the assignment form keeps its values after saving; the recap does not
  refresh after a regrade; the student assignment page shows two pagination bars and a stale status; a
  failed visible test case shows an empty output.
- **Owner: Erlangga31, with review by whosblub and Melvyn Faith.** Merged on 12 Nov:
  [PR #117](https://github.com/MelvynFaith/sandboxed-autograder/pull/117) (44 px touch targets),
  [PR #113](https://github.com/MelvynFaith/sandboxed-autograder/pull/113) (the lecturer flow from Course
  to recap and CSV, with the UAT scenario of SRS 11.5 in `docs/uat/`: five test cases, one hidden, and
  ten different programs with their expected scores),
  [PR #114](https://github.com/MelvynFaith/sandboxed-autograder/pull/114) (the NFR-11 audit) and
  [PR #137](https://github.com/MelvynFaith/sandboxed-autograder/pull/137) (notes on the student and
  lecturer flows, as input for the user manual).
- **Owner: Melvyn Faith.** Every pull request of the week is merged, including the sprint 3 review and
  the week 11 and 12 notes ([PR #121](https://github.com/MelvynFaith/sandboxed-autograder/pull/121),
  [#125](https://github.com/MelvynFaith/sandboxed-autograder/pull/125)), the preliminary load test result
  ([PR #122](https://github.com/MelvynFaith/sandboxed-autograder/pull/122)) and the removal of the
  departed member from CODEOWNERS ([PR #126](https://github.com/MelvynFaith/sandboxed-autograder/pull/126)).

## In progress / carried over

- **Owner: Erlangga31.** Issues #129 to #134 (the defects above) are open and Erlangga31 has been told.
  [Issue #111](https://github.com/MelvynFaith/sandboxed-autograder/issues/111) stays open until the UAT
  run confirms the lecturer flow, and [issue #112](https://github.com/MelvynFaith/sandboxed-autograder/issues/112)
  (NFR-11) until the manual keyboard and device checks that the audit lists as unverified are done.
- **Owner: Melvyn Faith.** The dashboard on the campus server is rebuilt and redeployed once the defects
  are fixed, followed by the smoke test through Nginx and a repeat of the browser test, so that the
  server runs one new build instead of two.
- **Owner: Robertus Gabeler.** The official load test and its report (WBS 6.2) have no recorded run; the
  preliminary result in `docs/qa` is not that run. The campus server is ready for it, on the build that
  will be released.
- **Owner: Melvyn Faith.** The release of `develop` to `main` with a tag (SRS 13.3), the user manual
  (WBS 7.2) and the BAST package (SRS 13.5). They follow the fixes of the open defects, the UAT, the
  official load test, no open `critical` issue and a green smoke test on the campus server, so that
  what is released and handed over is the finished program.

## Blockers and risks

- **R6, owner: Melvyn Faith.** The UAT with the instructor has no confirmed date, and the QA window
  ended on 9 Nov. The deployment window ends on 23 Nov.
- **Owner: project team.** The UAT run needs an isolated database (SRS 11.5 and `docs/uat/`). The last
  attempt, by the PostgreSQL role that was at hand, was refused because that role cannot create a
  database ([PR #113](https://github.com/MelvynFaith/sandboxed-autograder/pull/113)). Until a database is
  provided, the backend part of the UAT scenario is not verified.
- **R10.** Reduced, not closed. The dashboard and `/healthz` were reached from outside the campus
  network (see Done), but there is still no DNS name, so there is no HTTPS, and passwords cross the
  network in plain text until there is.
- **Owner: project team.** SRS 13.1 puts the load test on staging, and there is no staging host. The
  campus server is the only host with the real build; the team has to decide whether the official run
  happens there, and when, so that it does not disturb the UAT.

## Next week

- Fix issues #129 to #134, then rebuild and redeploy the dashboard on the campus server, run the smoke
  test through Nginx and repeat the browser test there (it was run on a local stack, not on the campus
  server).
- Provide an isolated database, confirm the UAT date with the instructor and hold the UAT; record the
  defects it finds, with the `critical` label where they apply.
- Run the official load test on the campus server and write its report; close
  [issue #94](https://github.com/MelvynFaith/sandboxed-autograder/issues/94) with its result, and
  [issue #90](https://github.com/MelvynFaith/sandboxed-autograder/issues/90) after the CSV is opened
  in a spreadsheet.
- Release `develop` to `main` with a tag, and hand over: user manual, BAST package, final SRS status
  (including the firewall and the R10 check).
- Write the week 14 note.
