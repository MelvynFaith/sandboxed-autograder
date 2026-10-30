# Week 10 progress (20 Oct - 26 Oct 2026)

**Phase / sprint:** Sprint 2: grading workflow, dashboard integration, and UAT preparation

> Team update recorded through 25 Oct 2026. Items are included only when their
> landing date is within this week; each contribution names its owner.

## Done

### Frontend and documentation

- **Owner: Erlangga31.** Added the Course and enrollment dashboard flow
  (FR-18, NFR-10; [PR #99](https://github.com/MelvynFaith/sandboxed-autograder/pull/99),
  merged 22 Oct).
- **Owner: Erlangga31.** Added test-case YAML import and export to the lecturer
  dashboard (FR-03; [PR #100](https://github.com/MelvynFaith/sandboxed-autograder/pull/100),
  merged 20 Oct).
- **Owner: Erlangga31.** Documented owned frontend and API behavior
  ([PR #101](https://github.com/MelvynFaith/sandboxed-autograder/pull/101),
  merged 20 Oct).
- **Owner: Erlangga31.** Completed lecturer assignment and test-case management,
  including resource limits and CRUD (FR-02, FR-03, FR-06;
  [PR #104](https://github.com/MelvynFaith/sandboxed-autograder/pull/104),
  merged 22 Oct).
- **Owner: Erlangga31.** Added the OpenAPI entries for authentication and Courses
  ([PR #108](https://github.com/MelvynFaith/sandboxed-autograder/pull/108)), documented every
  implemented route ([PR #115](https://github.com/MelvynFaith/sandboxed-autograder/pull/115)), and
  made the dashboard accept status-only polling responses
  ([PR #110](https://github.com/MelvynFaith/sandboxed-autograder/pull/110)); all merged 24 Oct.
- **Owner: Melvyn Faith.** Dashboard pages on the real API, merged 20 Oct: the resource limits sent
  when an assignment is created ([PR #70](https://github.com/MelvynFaith/sandboxed-autograder/pull/70)),
  the administration page with users, queue, rate limits and the audit log
  ([PR #71](https://github.com/MelvynFaith/sandboxed-autograder/pull/71)), submission history and
  reports with cancel ([PR #72](https://github.com/MelvynFaith/sandboxed-autograder/pull/72)), and the
  code view, grade recap with CSV export and regrade
  ([PR #98](https://github.com/MelvynFaith/sandboxed-autograder/pull/98)).

### Backend and scheduler

- **Owner: Melvyn Faith.** Added the endpoint that returns a submission's source code to its owner, the
  Course's lecturer and administrators (FR-20, NFR-13;
  [PR #74](https://github.com/MelvynFaith/sandboxed-autograder/pull/74), merged 20 Oct) and the YAML
  import and export of test cases (FR-03; [PR #87](https://github.com/MelvynFaith/sandboxed-autograder/pull/87),
  merged 20 Oct).
- **Owner: Melvyn Faith.** Merged 22 Oct: assignment titles and a hidden marker on results in the
  submission and report responses, which the dashboard needed
  ([PR #68](https://github.com/MelvynFaith/sandboxed-autograder/pull/68)); the grade recap and its CSV
  export (FR-11; [PR #69](https://github.com/MelvynFaith/sandboxed-autograder/pull/69)); and the
  administrator endpoints for the audit log, the queue and the rate limits (FR-12, FR-13, FR-14;
  [PR #73](https://github.com/MelvynFaith/sandboxed-autograder/pull/73)).

### QA and UAT

- **Owner: Melvyn Faith.** Reviewed [PR #99](https://github.com/MelvynFaith/sandboxed-autograder/pull/99),
  [PR #113](https://github.com/MelvynFaith/sandboxed-autograder/pull/113) and
  [PR #114](https://github.com/MelvynFaith/sandboxed-autograder/pull/114) and asked for changes on each.
- No UAT with the instructor and no load test took place this week, and no defect was recorded. The
  UAT scenario was only prepared (Erlangga31, PR #113, not merged).

### Project and operations

- At the feature freeze on 26 Oct every functional requirement FR-01 to FR-20 had its backend on
  `develop`. Responsiveness and accessibility (NFR-11) were not complete
  ([issue #112](https://github.com/MelvynFaith/sandboxed-autograder/issues/112)), the UAT had not taken
  place, and no deployment after 19 Oct is recorded.

## In progress / carried over

- **Owner: Erlangga31.** [PR #106](https://github.com/MelvynFaith/sandboxed-autograder/pull/106)
  updates this team note and component READMEs; review changes are being addressed.
- **Owner: Erlangga31.** [PR #113](https://github.com/MelvynFaith/sandboxed-autograder/pull/113)
  connects the lecturer UAT workflow; the full SRS 11.5 scenario and zero-intervention
  evidence remain in progress.
- **Owner: Erlangga31.** [PR #114](https://github.com/MelvynFaith/sandboxed-autograder/pull/114)
  records the NFR-11 audit; reviewer follow-up is pending.
- **Owner: Erlangga31.** [PR #116](https://github.com/MelvynFaith/sandboxed-autograder/pull/116)
  implements keyboard focus trapping for confirmation dialogs and awaits review.
- **Owner: Erlangga31.** [PR #117](https://github.com/MelvynFaith/sandboxed-autograder/pull/117)
  raises measured touch targets to 44 CSS px and awaits review.
- **Owner: unassigned.** [Issue #9](https://github.com/MelvynFaith/sandboxed-autograder/issues/9)
  remains open for confirmation of frontend API contracts.
- **Owner: unassigned.** [Issue #60](https://github.com/MelvynFaith/sandboxed-autograder/issues/60)
  remains open to confirm YAML import/export acceptance criteria.

## Blockers and risks

- **Owner: Erlangga31.** The isolated-database lecturer UAT and its full SRS 11.5
  evidence are not complete; do not use results from `autograder_dev` as UAT proof.
- **Owner: project team.** Confirm the 27 Oct UAT schedule and any scope deferrals
  before the 26 Oct feature freeze.

## Next week

- Complete the isolated-database lecturer UAT, record measured interventions,
  and resolve or document any blocked SRS 11.5 criteria.
- Resolve API-contract and YAML acceptance questions in issues #9 and #60.
- Record each component's week-10 contribution above with its owner and repository
  reference before closing the weekly update.
