# Week 09 progress (13–19 Oct 2026)

**Phase / sprint:** Frontend SRS alignment and release readiness

## Done

- [PR #19](https://github.com/MelvynFaith/sandboxed-autograder/pull/19) — Frontend SRS alignment, admin/audit panel, and review fixes merged to `develop` on 15 Oct. Owner: Benedictus Erlangga.
- Frontend lint, 64 tests, and production build passed locally; the PR frontend CI check passed.

## In progress / carried over

- Confirm backend implementation ownership and API payload contracts with the team; frontend API response shapes remain assumptions until backend confirmation.
- Implement the FR-11 grade-summary/CSV frontend UI; confirm its backend endpoint
  contract and backend owner with the team.

## Blockers and risks

- `scheduler/`, `grader/`, and `infra/` currently contain only README files; backend availability and delivery ownership need confirmation before UAT.
- UAT date confirmation is overdue (R6); reconfirm the schedule with the instructor/assistants.
- Feature-freeze is 26 Oct and UAT starts 27 Oct; the team needs to agree which Should/Could requirements can be deferred and update SRS ownership/API contracts.
- Confirm who will back up the sandbox work after Evan's departure.

## Next week

- Ask Robertus and Melvyn whether backend code exists outside the repository and confirm a scheduler/API owner.
- Agree with the team on Should/Could scope cuts and the sandbox backup; confirm
  the backend owner and API contract for FR-11.
- Reconfirm the UAT schedule and update SRS ownership and JSON API contracts.
- Prioritize the next frontend issue only after scope and backend dependencies are agreed.
