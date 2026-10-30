# Week 11 progress (27 Oct - 2 Nov 2026)

**Phase / sprint:** QA, security testing and UAT (WBS 6)

> State on Friday 30 Oct 2026, written by Melvyn Faith from the repository and GitHub. It is to be
> updated before the week closes on 2 Nov. Dates are GitHub's merge dates, in UTC.

## Done

- **Owner: Erlangga31.** Confirmation dialogs trap keyboard focus
  ([PR #116](https://github.com/MelvynFaith/sandboxed-autograder/pull/116), merged 27 Oct) and the
  week 10 note and owned API documentation were refreshed
  ([PR #106](https://github.com/MelvynFaith/sandboxed-autograder/pull/106), merged 27 Oct).
- **Owner: Melvyn Faith.** Checks on `develop` at `62908c7`, run on 30 Oct on a local stack with the
  real Harness and sandbox (details in [PR #120](https://github.com/MelvynFaith/sandboxed-autograder/pull/120)):
  - `go test -race ./...` passes in every package; Go statement coverage is 77.2% across packages
    (`-coverpkg=./...`).
  - The UAT scenario of SRS 11.5 through the REST API (ten different programs, five test cases, two
    hidden): all ten automatic scores equal the hand-made ones, the lecturer read every program and
    exported the CSV, and no student saw hidden content. This is not the UAT with the instructor.
  - The sandbox adversarial suite passes 64 of 64 on the normal build and on the ptrace fallback; 28
    API-side attack checks pass (IDOR, cross-class access, role escalation, sessions, hidden test
    cases, a 200-upload flood); a scan of `/proc` during grading found no test case content.
  - Failure injection (NFR-04): a backend and sandbox killed while a submission was `RUNNING` came
    back, requeued it and graded it, with nothing left behind. The sweep of leftovers under a
    persistent unit cgroup is not covered; the commands for the campus server are in the report.
- **Owner: Melvyn Faith.** Opened three pull requests, all awaiting review:
  [#118](https://github.com/MelvynFaith/sandboxed-autograder/pull/118) (the Nginx site that serves
  the dashboard, and a smoke test that uploads through the API; the dashboard loaded through Nginx in
  a browser with no Content-Security-Policy violation),
  [#119](https://github.com/MelvynFaith/sandboxed-autograder/pull/119) (changelog catch-up) and
  [#120](https://github.com/MelvynFaith/sandboxed-autograder/pull/120) (the adversarial report and its
  scripts, WBS 6.1).
- **Owner: Melvyn Faith.** The [week 10 note](minggu-10.md) sections that were empty are filled, and
  the [sprint 3 review](../sprint/sprint-3.md) is written.

## In progress / carried over

- **Owner: Erlangga31.** [PR #117](https://github.com/MelvynFaith/sandboxed-autograder/pull/117)
  (44 px touch targets) is approved; [PR #114](https://github.com/MelvynFaith/sandboxed-autograder/pull/114)
  (the NFR-11 audit) is approved but conflicts with `develop`; the manual keyboard checks of NFR-11
  are open ([issue #112](https://github.com/MelvynFaith/sandboxed-autograder/issues/112)).
- **Owner: Erlangga31.** [PR #113](https://github.com/MelvynFaith/sandboxed-autograder/pull/113), the
  lecturer UAT flow and scenario, has changes requested: the ten programs of the scenario have to
  differ so that the score comparison tests something, and one test case should be hidden.
- **Owner: Robertus Gabeler.** The load test and its report (WBS 6.2): the k6 scripts are in
  `infra/loadtest/` and have not been run.
- **Owner: Melvyn Faith.** The deployment of `develop` to the campus server, including the dashboard
  behind Nginx, after #118 merges; the UAT data on the server; the release of `develop` to `main`.

## Blockers and risks

- **R6, owner: Melvyn Faith.** The UAT date is not confirmed with the instructor. The window of 27
  Oct to 9 Nov ends in ten days.
- **R10.** The backend on the campus server listens on loopback only. Nginx, a DNS name for HTTPS and
  the proof of access from outside the campus network are not done.
- **Owner: project team.** The campus server runs the build from before the submission API, so the
  adversarial API checks, the smoke test through the upload endpoint and the UAT cannot run there yet.
- **Owner: project team.** SRS 13.1 puts the load test on staging, and there is no staging host: the
  team has to decide where the official run happens.
- **Owner: project team.** The GitHub Projects board is out of date (most cards still say Review or
  In Progress although their pull requests are merged) and the `critical` label of SRS 10.4 does not
  exist, so the defect indicator cannot be queried.

## Next week

- Hold the UAT with the instructor, record the interventions and the defects, and fix what it finds.
- Deploy to the campus server and run the smoke test, the API attack checks and the crash test there.
- Run the load test and write its report; confirm the fairness criterion of
  [issue #94](https://github.com/MelvynFaith/sandboxed-autograder/issues/94).
- Merge the NFR-11 pull requests and finish the keyboard checks.
- Prepare the release of `develop` to `main` with a tag, and update the SRS to the present dates and
  status.
