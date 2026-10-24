# Week 10 progress (20 Oct - 26 Oct 2026)

**Phase / sprint:** Backend critical path to submission flow and UAT preparation

## Done

- B3 assignment and test-case APIs merged in [PR #59](https://github.com/MelvynFaith/sandboxed-autograder/pull/59) on 19 Oct; issue #58 is closed. The API requires all six resource limits when creating an assignment and preserves test cases used by any submission.
- B4 submission upload, history, detail, and report APIs merged in [PR #63](https://github.com/MelvynFaith/sandboxed-autograder/pull/63) on 19 Oct; issue #61 is closed. It is no longer awaiting review.
- Authentication and profile UI/API work in [PR #54](https://github.com/MelvynFaith/sandboxed-autograder/pull/54) and [PR #35](https://github.com/MelvynFaith/sandboxed-autograder/pull/35) merged on 19 and 17 Oct.
- Course/enrollment API [PR #51](https://github.com/MelvynFaith/sandboxed-autograder/pull/51) merged on 19 Oct; Course UI [PR #99](https://github.com/MelvynFaith/sandboxed-autograder/pull/99) merged on 22 Oct.
- Lecturer assignment and test-case UI [PR #104](https://github.com/MelvynFaith/sandboxed-autograder/pull/104) merged on 22 Oct. YAML import/export API [PR #87](https://github.com/MelvynFaith/sandboxed-autograder/pull/87) and UI [PR #100](https://github.com/MelvynFaith/sandboxed-autograder/pull/100) merged on 20 Oct.
- [PR #68](https://github.com/MelvynFaith/sandboxed-autograder/pull/68), [PR #69](https://github.com/MelvynFaith/sandboxed-autograder/pull/69), and [PR #73](https://github.com/MelvynFaith/sandboxed-autograder/pull/73) were reviewed by Erlangga31 on 22 Oct 2026 and subsequently merged. Erlangga31 has no review recorded on [PR #87](https://github.com/MelvynFaith/sandboxed-autograder/pull/87); that PR was merged on 20 Oct after a review by whosblub.
- GitHub reports no open PRs authored by Erlangga31. All of the author's PRs returned by `gh pr list --author Erlangga31 --state all` are merged:
  - #104 — lecturer assignment and test-case screens — 22 Oct 2026.
  - #101 — Erlangga-owned frontend and API documentation — 20 Oct 2026.
  - #100 — YAML test-case UI — 20 Oct 2026.
  - #99 — Course and enrollment UI — 22 Oct 2026.
  - #63 — submission API — 19 Oct 2026.
  - #59 — assignment and test-case API — 19 Oct 2026.
  - #54 — registration, profile, and logout UI — 19 Oct 2026.
  - #51 — Course and enrollment API — 19 Oct 2026.
  - #40 — Week 10 progress note — 18 Oct 2026.
  - #35 — authentication and session API — 17 Oct 2026.
  - #32 — registration and profile UI — 17 Oct 2026.
  - #22 — frontend submission contract fixes — 17 Oct 2026.
  - #20 — Week 09 progress note — 16 Oct 2026.
  - #19 — admin dashboard UI — 15 Oct 2026.
  - #16 — lecturer assignment management UI — 13 Oct 2026.
  - #14 — submission history and report UI — 5 Oct 2026.
  - #12 — live submission status UI — 5 Oct 2026.
  - #10 — student assignment list and upload UI — 5 Oct 2026.
  - #7 — login and role-based routing UI — 4 Oct 2026.
  - #4 — frontend scaffold — 4 Oct 2026.

## In progress / carried over

- No open PR by Erlangga31 is reported by GitHub. Issue [#60](https://github.com/MelvynFaith/sandboxed-autograder/issues/60) remains open although YAML import/export is implemented in merged PRs #87 and #100; review its remaining acceptance criteria before deciding whether to close it.
- Issue [#9](https://github.com/MelvynFaith/sandboxed-autograder/issues/9) remains open for confirmation of temporary frontend API-contract assumptions.

## Blockers and risks

- R6: the proposed UAT window is 27 Oct–9 Nov, but no schedule confirmation is recorded. The PM must confirm the schedule or decide whether to use the SRS fallback; the status remains unresolved.
- R4/R7: replacement sandbox-backup and Module C reviewer coverage has not been confirmed in the latest project record; status is **belum diverifikasi**.
- Feature freeze is 26 Oct. There is no recorded decision on which remaining Should/Could requirements, if any, will be deferred; resolve this before freeze. The former FR-11 ownership/API decision is no longer listed as a blocker: the backend and dashboard PRs #69 and #98 have merged.

## Next week

- Confirm the UAT schedule and prepare role-based test accounts and assignment scenarios.
- Resolve the open contract questions in issue #9 and reconcile the remaining acceptance criteria of issue #60 with the merged YAML implementation.
- Confirm whether R4/R7 replacement coverage is needed and, if so, assign it before UAT.
