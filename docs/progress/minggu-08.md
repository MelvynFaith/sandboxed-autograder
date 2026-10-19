# Week 08 progress (6 Oct – 12 Oct 2026)

**Phase / sprint:** Sprint 2: integration and grading (WBS 4, weeks 7–8)

> Written on 19 Oct 2026 by Melvyn Faith from the repository history and the SRS revision record.
> Only items with a record are listed.

## Done

- Sandbox Executor `sandbox-exec` (WBS 3.1, 3.2; FR-05, FR-06, FR-07), pushed on 10 Oct as
  [PR #17](https://github.com/MelvynFaith/sandboxed-autograder/pull/17). It covers user, PID,
  mount, network, UTS and IPC namespaces, cgroup v2 limits, a seccomp allowlist that reports the
  denied syscall number, a locked-down environment and a separate JSON result channel. The PR also
  added the adversarial test suite, C style rules and the module documentation. It was merged on
  16 Oct. Owner: Melvyn Faith.
- SRS version 1.3 (10 Oct), the week 8 consistency check of 1.2. It defined the `sandbox-exec`
  contract between the Harness and the sandbox (SRS 3.2.1.7), corrected the mount layout to what
  the kernel allows, and closed a leak of hidden test cases through the student report. It also
  recorded the sandbox's measured fixed cost: about 4 ms per run on WSL2 (100 runs of
  `/usr/bin/true`), against the 50 ms assumed in the NFR-01a model.

## In progress / carried over

- [PR #16](https://github.com/MelvynFaith/sandboxed-autograder/pull/16), lecturer assignment
  management in the dashboard (opened 5 Oct), under review; merged on 13 Oct. Owner: Benedictus
  Erlangga.
- [Issue #9](https://github.com/MelvynFaith/sandboxed-autograder/issues/9), the temporary
  frontend API contracts, still open: the backend had no endpoints yet.
- Sprint 2 backend and Harness packages carry over to week 9: the Harness (WBS 4.1), its
  integration with the sandbox (WBS 4.2), the job queue, worker pool and rate limits (WBS 4.3), and
  submission upload (WBS 4.4). The `scheduler/` and `grader/` directories had no code yet.

## Blockers and risks

- R6: the deadline to confirm the UAT schedule with the instructor (12 Oct, end of week 8) passed
  without a confirmation.
- The Sprint 2 milestone (end-to-end flow from upload to a grade on the dashboard) was missed: no
  backend or Harness code existed yet.
- R2, R9, R10, R11 and R12 were still unverified on the campus server.

## Next week

- Merge the sandbox, verify the campus server, and build the scheduler and the Harness.
