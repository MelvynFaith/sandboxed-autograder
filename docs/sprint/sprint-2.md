# Sprint 2 review and retrospective

**Dates:** 29 Sep – 12 Oct 2026 (weeks 7–8)
**Sprint goal:** the end-to-end submission flow (upload, run in the sandbox, grade shown on the
dashboard): dashboard integration with the API, real-time status over WebSocket, and the Grading
Harness integrated with the sandbox (WBS 4).

> Written on 19 Oct 2026 by Melvyn Faith, PM from that date, from the repository history, the week
> 07 note and the SRS revision record. No review meeting was recorded for this sprint. The
> retrospective below is drawn from what the record shows; teammates are welcome to add to it.

## Review

### Delivered

| Item | Requirements | Owner |
|---|---|---|
| Frontend scaffold, login and role-based routing ([#4](https://github.com/MelvynFaith/sandboxed-autograder/pull/4), [#7](https://github.com/MelvynFaith/sandboxed-autograder/pull/7)) | FR-01 (UI) | Benedictus Erlangga |
| Assignment list and Python upload, live submission status, history and test reports ([#10](https://github.com/MelvynFaith/sandboxed-autograder/pull/10), [#12](https://github.com/MelvynFaith/sandboxed-autograder/pull/12), [#14](https://github.com/MelvynFaith/sandboxed-autograder/pull/14)), built against temporary API contracts ([#9](https://github.com/MelvynFaith/sandboxed-autograder/issues/9)) and mock data | FR-04, FR-08, FR-09, FR-10 (UI) | Benedictus Erlangga |
| Sandbox Executor and adversarial suite ([#17](https://github.com/MelvynFaith/sandboxed-autograder/pull/17), opened 10 Oct, merged 16 Oct) | FR-05, FR-06, FR-07, FR-12, NFR-02, NFR-03 | Melvyn Faith |
| SRS 1.3 (10 Oct): `sandbox-exec` contract, mount layout fixed, hidden test-case leak in the report closed, sandbox overhead measured at about 4 ms per run | FR-03, FR-05, NFR-08 | Not recorded |

### Not delivered

| Item | Reason | Plan |
|---|---|---|
| Grading Harness: test-case iteration and output comparison (WBS 4.1) | Not started in the sprint | Merged in week 9: [#26](https://github.com/MelvynFaith/sandboxed-autograder/pull/26), [#33](https://github.com/MelvynFaith/sandboxed-autograder/pull/33), [#44](https://github.com/MelvynFaith/sandboxed-autograder/pull/44), [#45](https://github.com/MelvynFaith/sandboxed-autograder/pull/45), [#48](https://github.com/MelvynFaith/sandboxed-autograder/pull/48) |
| Harness and sandbox integration (WBS 4.2) | Both sides were missing | Merged in week 9: [#29](https://github.com/MelvynFaith/sandboxed-autograder/pull/29) (Go side), [#45](https://github.com/MelvynFaith/sandboxed-autograder/pull/45) (Python side) |
| Job queue, worker pool, fair queueing, rate limits (WBS 4.3; FR-13, FR-14) | No scheduler code yet | Merged in week 9: [#27](https://github.com/MelvynFaith/sandboxed-autograder/pull/27), [#31](https://github.com/MelvynFaith/sandboxed-autograder/pull/31), [#36](https://github.com/MelvynFaith/sandboxed-autograder/pull/36) |
| Submission upload and late policy (WBS 4.4; FR-04) | No scheduler code yet | [#63](https://github.com/MelvynFaith/sandboxed-autograder/pull/63), open since 19 Oct |
| Real-time status from the backend (WBS 4.5; FR-08) | No scheduler code yet | WebSocket stream merged in week 9 ([#49](https://github.com/MelvynFaith/sandboxed-autograder/pull/49)) |
| Lecturer assignment management UI ([#16](https://github.com/MelvynFaith/sandboxed-autograder/pull/16)) | Still in review at the end of the sprint | Merged 13 Oct |

### Demo notes

- No demo was recorded. The frontend could be shown with mock data only; the sprint milestone, an
  end-to-end submission flow, was not met.

## Retrospective

### What went well

- The frontend kept moving without a backend: mock data behind `VITE_USE_MOCK` and the temporary
  contracts in issue #9 let the scaffold and four features land in week 7.
- The sandbox arrived with its adversarial suite and a man page, so the Harness had a fixed
  contract to call.

### What to improve

- The backend and the Harness started two sprints late, and all integration fell into week 9.
  Many interdependent PRs were then opened on the same days, which produced merge conflicts and
  repeated rebases.
- Building pages against assumed contracts cost later rework: the dashboard had to be realigned
  with the SRS and the real API ([#19](https://github.com/MelvynFaith/sandboxed-autograder/pull/19),
  [#22](https://github.com/MelvynFaith/sandboxed-autograder/pull/22)).
- The UAT schedule was not confirmed by its 12 Oct deadline (R6).

### Action items

| Action | Owner | Due |
|---|---|---|
| Confirm the UAT schedule with the instructor (R6) | Melvyn Faith (PM) | As soon as possible, before 27 Oct |
| Publish the API contract as `docs/openapi.yaml` so the frontend works from it instead of assumptions | Benedictus Erlangga (his endpoints), Melvyn Faith (pipeline endpoints) | Before 26 Oct |
| Keep parallel PRs independent and mergeable: no stacked branches, separate changelog hunks | Everyone | Ongoing |
