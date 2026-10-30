# Preliminary load test (SRS 11.4, WBS 6.2)

**Date:** 30 Oct 2026. **Author:** Melvyn Faith. **Build:** `develop` at `62908c7`.

This is a **preliminary** run of the k6 scripts in `infra/loadtest/` on a development machine, to see
early whether the targets of NFR-01a and NFR-01b are in reach and whether the scripts work against the
real upload endpoint. It is **not** the official test: SRS 13.1 puts that on staging, which does not
exist yet, and the numbers below come from a different machine from the campus server. The official run
and its report are still WBS 6.2.

## Setup

| | |
|---|---|
| Machine | WSL2 on Windows, 20 logical CPUs, 3.7 GB of RAM, kernel 6.18.33.2; PostgreSQL 18, k6 v2.3.0 and the backend on the same host |
| Backend | the build at `62908c7` with the real Grading Harness and `sandbox-exec`, `WORKERS=4` (the W of SRS 5.1), started in its own systemd scope with a delegated cgroup |
| Data | a fresh database; `infra/loadtest/seed.sh` registered 100 students through the API and made the `LOAD` course with the assignment every account submits to and 30 flood assignments; each has the five test cases of the SRS 5.1 model |
| Program | every account submits the same program, `print(int(input()) * 2)`, which scores 100 |
| Method | before each scenario the backend was restarted (a new cgroup, so `memory.peak` is that scenario's own) and the load-test submissions were reset; then `k6 run`, then `seed.sh report` |

## Results

All seven scenarios ended with exit status 0, which means their k6 thresholds held. Every submission
ended `COMPLETED` with 100 of 100, and the backend logged no `ERROR` line.

| Scenario | Requirement | End-to-end average | median | p95 | maximum | Backend CPU | `memory.peak` |
|---|---|---|---|---|---|---|---|
| 5 accounts at once | NFR-01a | 253 ms | 225 ms | 339 ms | 367 ms | 1.5 s | 194 MB |
| 10 accounts | NFR-01a | 281 ms | 302 ms | 440 ms | 441 ms | 2.5 s | 196 MB |
| 30 accounts | NFR-01a | 614 ms | 588 ms | 1.06 s | 1.12 s | 6.8 s | 204 MB |
| 60 accounts | NFR-01b | 1.21 s | 1.22 s | 2.21 s | 2.21 s | 14.0 s | 216 MB |
| 100 accounts | NFR-01b | 2.30 s | 2.36 s | 4.04 s | 4.17 s | 28.2 s | 226 MB |

End-to-end means from the start of `POST /api/v1/submissions` to the final WebSocket frame. The server's
own time from `submitted_at` to `graded_at` was close to it (0.22 s, 0.26 s, 0.59 s, 1.19 s and 2.23 s
on average). "Backend CPU" is the CPU time of the backend's cgroup, which includes the Harness and
the sandboxes; "memory.peak" is that cgroup's highest use.

- **NFR-01a** asks for an average of at most 10 s with up to 30 simultaneous submissions. Measured: 0.61 s
  at 30. The model of SRS 5.1 predicts about 5.6 s at 30 (W = 4, 150 ms per test case, five test
  cases). The measured time of one submission when workers are free is at most about 0.2 s (5 accounts: 0.22 s
  on the server), against 0.75 s in the model, so the assumed 150 ms per test case is about three times
  too high on this machine. The SRS asks that when measurement and model differ widely the target
  figure is revised, not the explanation; here the model is conservative, and the number to revisit
  is the per-test-case cost, after the official run.
- **NFR-01b** asks that 100 simultaneous submissions neither crash nor lose data, with the p95 reported
  as a baseline: all 100 completed, p95 4.04 s, maximum 4.17 s, with 226 MB of peak memory in the
  backend's cgroup.
- **FR-14** (`ratelimit.js`): one account sent 200 submissions in a row; 1 was accepted and 199 were
  refused with HTTP 429 and a reason (38 because a submission was still queued, 161 because the 30
  seconds had not passed), and the 10 other accounts that submitted meanwhile were graded in 314 ms on
  average (limit 10 s).
- **FR-13** (`fairness.js`): one account queued 30 submissions, one to each flood assignment, and then 10
  other accounts submitted. The baseline of the 10-account run was 280 ms, so the limit was 560 ms; the
  10 accounts averaged 409 ms (1.46 times the baseline), p95 533 ms.

## What this does not show

1. **A different machine.** This host has 20 logical CPUs and the four workers never competed for
   cores; the campus server's CPU count and speed are not recorded here. The model takes W as the
   server's core count. The official run on the target server may be slower.
2. **k6, PostgreSQL and the backend shared one host**, as they would on the campus server if k6 ran
   there (the README of `infra/loadtest/` notes the same).
3. **One tiny program.** Every submission is the same one-line program; real submissions of up to
   200 lines, or ones that use their time limit, take longer. The SRS excludes the latter from NFR-01a.
4. **No Nginx and no dashboard** in the path: k6 spoke to the backend directly, including the
   WebSocket.
5. **The fairness baseline is small** (280 ms), so a ratio of 1.46 is a few hundred milliseconds; the
   official run on a slower server gives a more telling ratio.
6. **WSL2 is not the target environment** for the sandbox (the campus server is Ubuntu 24.04), so the
   cost of creating a sandbox may differ.

## Next

- Robertus Gabeler (WBS 6.2) runs the official test on the host the team decides on (SRS 13.1 names
  staging; there is none) and writes its report, with the server's CPU and memory as the README asks.
- Until then the figures above can be quoted as a preliminary local result only.
- After the official run, revise the per-test-case cost in SRS 5.1 if it confirms the difference.
