# Week 06 progress (22 Sep – 28 Sep 2026)

**Phase / sprint:** Sprint 1: sandbox core and base API (WBS 3, weeks 5–6)

> Written on 19 Oct 2026 by Melvyn Faith from the repository history and the SRS revision record.
> Only items with a record are listed.

## Done

- SRS version 1.1 (22 Sep), from the internal review before Sprint 2 integration. Among other
  things it fixed the call direction between modules (the Grading Harness calls the sandbox, the
  scheduler is the only database writer), specified the sandbox privilege model and a separate
  result channel, added FR-14 (submission rate limits), NFR-10 and NFR-11, completed the endpoint
  table, added the ERD and the traceability matrix, and added risks R10 and R11.
- SRS version 1.2 (23 Sep), from the technical review of 1.1. It corrected kernel mechanisms that
  could not have been implemented as written (`cpu.max` is a bandwidth limit, not a CPU-time
  budget; `SCMP_ACT_TRAP` cannot report the denied syscall to the parent, so `SCMP_ACT_NOTIFY` with
  an `SCMP_ACT_TRACE` fallback replaced it). It also added Course and Enrollment, FR-15 to FR-20,
  NFR-12 and NFR-13, the staging environment, the user stories with acceptance criteria, the WBS,
  and the sequence and state diagrams.
- Repository created on 23 Sep with README, contribution guide, security policy, changelog,
  CODEOWNERS, issue and PR templates, the templates for these weekly and sprint notes, one README
  per component, and the GitHub Actions CI (WBS 2.5). Owner: Melvyn Faith.
- The CI gate was checked with three throwaway pull requests, closed unmerged: the
  `pull_request` trigger (#1), a failing check blocking the merge (#2), and an empty diff (#3).
  Owner: Melvyn Faith.

## In progress / carried over

- Every Sprint 1 code package (WBS 3.1–3.6) carries over; none had code in the repository. They
  arrived in weeks 8–9: the sandbox executor in PR #17 (opened 10 Oct), the schema in PR #25,
  authentication in PR #35, and Course and Enrollment in PR #51.
- Staging (WBS 2.6) and the campus server verification (WBS 2.4).

## Blockers and risks

- The Sprint 1 checkpoint was missed: by the end of week 6 the sandbox did not yet run code in the
  repository, there was no authentication API, and no database ran on staging.

## Next week

- Sprint 2 (weeks 7–8): integration and grading (WBS 4).
