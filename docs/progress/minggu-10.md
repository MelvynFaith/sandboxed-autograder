# Week 10 progress (20 Oct - 26 Oct 2026)

**Phase / sprint:** Sprint 2: grading workflow, dashboard integration, and UAT preparation

> Status recorded through 24 Oct 2026. Only work and decisions with a repository or GitHub record are included.

## Done

- The lecturer workflow for Courses and enrollments is available in the dashboard. Lecturers can manage Courses and enroll students; students see their enrolled Courses and course assignments (FR-18, NFR-10; [PR #99](https://github.com/MelvynFaith/sandboxed-autograder/pull/99)).
- Lecturer assignment and test-case management is integrated, including required resource limits, CRUD, and paginated test-case lists (FR-02, FR-03, FR-06; [PR #104](https://github.com/MelvynFaith/sandboxed-autograder/pull/104)).
- Test-case YAML import/export is available through the API and lecturer dashboard using schema version 1 (FR-03 Must; [PR #87](https://github.com/MelvynFaith/sandboxed-autograder/pull/87), [PR #100](https://github.com/MelvynFaith/sandboxed-autograder/pull/100)).
- Submission history and reports use the API; lecturers and administrators can view assignment grade recaps and export CSV, access submission code, and request regrades (FR-08, FR-09, FR-11, FR-17, FR-20; [PR #69](https://github.com/MelvynFaith/sandboxed-autograder/pull/69), [PR #72](https://github.com/MelvynFaith/sandboxed-autograder/pull/72), [PR #74](https://github.com/MelvynFaith/sandboxed-autograder/pull/74), [PR #98](https://github.com/MelvynFaith/sandboxed-autograder/pull/98)).
- The admin dashboard now includes audit-log and rate-limit controls (FR-12, FR-14; [PR #71](https://github.com/MelvynFaith/sandboxed-autograder/pull/71), [PR #73](https://github.com/MelvynFaith/sandboxed-autograder/pull/73)).
- Erlangga31 reviewed PRs #68, #69, and #73 on 22 Oct; each was subsequently merged. No review by Erlangga31 is recorded on PR #87, which was merged after review by whosblub.

## In progress / carried over

- The frontend polling fallback for status-only submission responses is under review in [PR #110](https://github.com/MelvynFaith/sandboxed-autograder/pull/110).
- [Issue #60](https://github.com/MelvynFaith/sandboxed-autograder/issues/60) remains open although YAML import/export is implemented; confirm whether any acceptance criteria remain before closing it.
- [Issue #9](https://github.com/MelvynFaith/sandboxed-autograder/issues/9) remains open for agreement on frontend API contracts. Its temporary login response differs from the current API response; update or close the assumption after agreement.

## Blockers and risks

- R6: the proposed UAT window of 27 Oct - 9 Nov is not confirmed. The PM must confirm the schedule or decide whether to use the SRS fallback before UAT starts.
- R4/R7: replacement sandbox-backup and Module C reviewer coverage has not been confirmed in the project records reviewed for this update.
- Feature freeze is 26 Oct. No decision on deferring remaining Should/Could scope is recorded; agree and communicate any cuts before the freeze.

## Next week

- Confirm the UAT schedule and prepare role-based test accounts and assignment scenarios.
- Resolve the outstanding contract and YAML acceptance questions in issues #9 and #60.
- Confirm reviewer and backup coverage for UAT and record any agreed scope deferrals.
