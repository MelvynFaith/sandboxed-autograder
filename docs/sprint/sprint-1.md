# Sprint 1 review and retrospective

**Dates:** 15 Sep – 28 Sep 2026 (weeks 5–6)
**Sprint goal:** a sandbox prototype that runs simple Python code under CPU and memory limits, the
authentication and profile API, the database schema on staging, and Course and Enrollment (WBS 3).

> Written on 19 Oct 2026 by Melvyn Faith, PM from that date, from the repository history and the
> SRS revision record. No review meeting was recorded for this sprint. The retrospective below is
> drawn from what the record shows; teammates are welcome to add to it.

## Review

### Delivered

| Item | Requirements | Owner |
|---|---|---|
| SRS 1.1 (22 Sep): internal review; call direction between modules, sandbox privilege model, separate result channel, endpoint table, ERD, traceability matrix | FR-14, NFR-10, NFR-11 added; R10, R11 added | Not recorded |
| SRS 1.2 (23 Sep): technical review; kernel mechanisms corrected, Course and Enrollment, user stories, WBS | FR-15–FR-20, NFR-12, NFR-13 added | Not recorded |
| Repository scaffolding, CODEOWNERS and CI, with the merge gate checked through PRs #1–#3 (23 Sep) | NFR-07 (CI) | Melvyn Faith |

### Not delivered

| Item | Reason | Plan |
|---|---|---|
| Sandbox prototype (WBS 3.1, 3.2) | Not recorded; no code in the repository during the sprint | Arrived in week 8: PR #17 (opened 10 Oct, merged 16 Oct) |
| Schema migration and database on staging (WBS 3.3) | No staging environment | Schema merged in PR #25 (17 Oct); staging still open |
| Authentication, session and profile API (WBS 3.4) | Not recorded | Merged in PR #35 (17 Oct) |
| Course and Enrollment (WBS 3.5) | Depended on 3.4 | Merged in PR #51 (19 Oct) |
| Sandbox knowledge-sharing session (WBS 3.6, R4) | Not recorded | Still open; the planned sandbox backup has since left the team |

### Demo notes

- No demo was recorded. The sprint checkpoint (the sandbox runs simple Python with CPU and memory
  limits, the authentication API works, the database runs on staging) was not met.

## Retrospective

### What went well

- The two SRS reviews caught design errors before any code depended on them. Version 1.2 replaced
  two kernel mechanisms that could not have worked as specified: `cpu.max` used as a CPU-time
  budget, and `SCMP_ACT_TRAP`, which cannot report the denied syscall to the supervisor (FR-12).
- CI and its merge gate were in place, and tested, before the first line of product code.

### What to improve

- Four weeks passed without a commit or a progress note, so the slip only became visible at the
  checkpoint.
- Server verification and staging (WBS 2.4, 2.6) were left behind, and Sprint 1 depended on them.

### Action items

| Action | Owner | Due |
|---|---|---|
| Write a progress note at the end of every week, from week 9 on | Melvyn Faith (PM) | Each Sunday |
| Verify the campus server (SRS table 6.1) | Melvyn Faith | Done 17 Oct |
| Set up staging, or decide where the load test and restore test run instead | Melvyn Faith | Before 27 Oct |
