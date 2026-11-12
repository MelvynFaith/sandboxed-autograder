# Week 12 progress (3 Nov - 9 Nov 2026)

**Phase / sprint:** QA, security testing and UAT (WBS 6)

> Written by Melvyn Faith on 12 Nov 2026, after the week had closed, from the repository and GitHub.
> It lists only what has a record. GitHub's event list and the branch dates show no commit, pull
> request, review or merge between 31 Oct and 11 Nov, so the sections below describe the state the
> week started and ended in. Work done outside the repository, if any, is not recorded here. Dates
> are GitHub's, in UTC.

## Done

- No change reached `develop` this week: the last merge was on 30 Oct
  ([PR #120](https://github.com/MelvynFaith/sandboxed-autograder/pull/120)), and nothing had merged
  by 12 Nov.
- The automatic dependency update check of the GitHub Actions workflows (Dependabot) ran on 4 Nov and
  opened no pull request.

## In progress / carried over

All of these were open during the whole week and unchanged at its close on 9 Nov.

- **Owner: Erlangga31.** [PR #117](https://github.com/MelvynFaith/sandboxed-autograder/pull/117) (44
  px touch targets) was approved; [PR #114](https://github.com/MelvynFaith/sandboxed-autograder/pull/114)
  (the NFR-11 audit) was approved but conflicted with `develop`; the manual keyboard checks of NFR-11
  ([issue #112](https://github.com/MelvynFaith/sandboxed-autograder/issues/112)) were open.
- **Owner: Erlangga31.** [PR #113](https://github.com/MelvynFaith/sandboxed-autograder/pull/113), the
  lecturer UAT flow and scenario, still had the changes requested on 27 Oct.
- **Owner: Robertus Gabeler.** The official load test and its report (WBS 6.2) had no recorded run;
  the preliminary run of 30 Oct on a development machine
  ([PR #122](https://github.com/MelvynFaith/sandboxed-autograder/pull/122)) is not the official one.
- **Owner: Melvyn Faith.** [PR #121](https://github.com/MelvynFaith/sandboxed-autograder/pull/121)
  (the sprint 3 review and the week 11 note) and
  [PR #122](https://github.com/MelvynFaith/sandboxed-autograder/pull/122) (the preliminary load test)
  were open without a review. The deployment of `develop` to the campus server was not done: it still
  ran the build of 19 Oct, without the submission API.

## Blockers and risks

- **R6, owner: Melvyn Faith.** The QA window of 27 Oct to 9 Nov ended without the UAT with the
  instructor: the date was never confirmed.
- **Owner: project team.** Five pull requests waited for a review or for rework through the whole
  week, which kept the NFR-11 work, the UAT scenario and the QA documents out of `develop`.
- **R10.** Nginx, HTTPS and the proof of access from outside the campus network were still not done.
- **Owner: project team.** SRS 13.1 puts the load test on staging, and there is no staging host: the
  team still has to decide where the official run happens.
- **Owner: Melvyn Faith.** The deployment window (weeks 13 and 14, 10 to 23 Nov) started with the
  campus server not yet upgraded, the UAT not held and the release not made.

## Next week

- Upgrade the campus server to `develop`, put the dashboard behind Nginx, and run the smoke test and
  the QA scripts there (done in week 13, see its note).
- Review and merge the open pull requests; rework #113 and rebase #114.
- Run the official load test and write its report.
- Confirm the UAT date with the instructor, then hold the UAT.
