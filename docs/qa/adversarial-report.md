# Adversarial test report (SRS 11.3, WBS 6.1) and failure injection (NFR-04)

**Date:** 30 Oct 2026 (week 11). **Author:** Melvyn Faith.
**Build tested:** `develop` at `62908c7`. **Host:** WSL2 Ubuntu 26.04, kernel 6.18.33.2, cgroup v2,
1 GiB of swap active, PostgreSQL 18, the real Grading Harness and `sandbox-exec`.

This report maps each of the 22 attack scenarios of SRS 11.3 to the evidence that exists, says what
was run today and what was not, and lists the gaps. A scenario is marked **pass** only when a test
or a run showed the expected result; **partial** when part of it was shown; **not run** otherwise.
Nothing here was run on the campus server: it still runs an older build (see the last section).

## What was run

| Run | Result |
|---|---|
| Sandbox adversarial suite, normal build (`SCMP_ACT_NOTIFY`), `sandbox/tests/run.sh` | 64 of 64 passed in 17 s |
| Sandbox adversarial suite, ptrace fallback build (`SBX_FORCE_TRACE`, SRS risk R12) | 64 of 64 passed in 16 s |
| API-side checks, [`infra/qa/adversarial_api.py`](../../infra/qa/adversarial_api.py), live backend | 28 of 28 passed |
| `/proc` leak probe, [`infra/qa/cmdline_probe.py`](../../infra/qa/cmdline_probe.py), live backend | no test case marker seen in about 6,000 scans per run, three runs; the control process was found each time |
| Failure injection (section below), live backend | the interrupted submission was recovered and graded, nothing left behind |
| UAT scenario driver (10 programs through the REST API), earlier the same day | 10 of 10 automatic scores equal the hand-made ones, no hidden-content leak |

The sandbox suite also runs in CI in three variants (normal, ptrace fallback, ASan/UBSan); the ASan
variant was not run locally today. The suite passed 64 of 64 on the campus server as a system service
in week 09 (week 09 note), on the build from before the submission API.

## Scenarios

| # | SRS 11.3 scenario | Evidence | Result |
|---|---|---|---|
| 1 | Fork bomb | `Limits.test_fork_bomb` | pass |
| 2 | CPU exhaustion | `Limits.test_cpu_exhaustion_is_cut_at_the_cpu_budget`; the UAT infinite-loop program ends as `TIMEOUT` through the whole pipeline | pass |
| 3 | Wall clock without CPU | `Limits.test_wall_clock_without_cpu` | pass |
| 4 | Memory bomb | `Limits.test_memory_bomb`; the UAT memory-bomb program ends as `MEMORY_LIMIT_EXCEEDED` | pass |
| 5 | Memory bomb via swap | `Limits.test_memory_bomb_cannot_escape_through_swap`; the host had swap (1 GiB), so the test is meaningful | pass |
| 6 | Peak memory of several processes | `Limits.test_peak_memory_covers_all_processes` | pass |
| 7 | Path traversal | `Isolation.test_path_traversal_sees_nothing_of_the_host`, `test_root_and_usr_are_read_only`, `test_every_host_bound_mount_is_read_only` | pass |
| 8 | Network exfiltration | `Syscalls.test_network_socket_is_denied_with_its_number`, `test_reverse_shell_attempt`; the UAT network program ends as `SECURITY_VIOLATION` | pass |
| 9 | Dangerous syscalls, logged with the number | `Syscalls.test_dangerous_syscalls`; the smoke test checks the `SANDBOX_VIOLATION` audit rows with syscall 41 | pass |
| 10 | Output bomb | `Limits.test_output_bomb_stdout`, `test_output_limit_is_combined_not_per_stream`, `test_output_exactly_at_the_limit_is_accepted` | pass |
| 11 | Output bomb via stderr | `Limits.test_output_bomb_stderr_counts_too` | pass |
| 12 | Disk-write bomb | `Limits.test_disk_write_bomb_hits_tmpfs_before_the_memory_cgroup`; the `tmpfs` invariant is refused by `Usage.test_tmpfs_must_exceed_twice_the_output_cap` and `test_tmpfs_needs_headroom_below_the_memory_limit`, and by the API at assignment creation (week 10 note) | pass |
| 13 | Submission flood | 200 uploads in a row from one student: all 200 refused with `429` (the account had a submission seconds earlier); another student's upload during the flood was accepted and graded in 0.5 s. The fairness load test with 10 other accounts (`fairness.js`, FR-13) was **not run** | partial |
| 14 | Forged result | `ResultChannel.test_forged_json_on_stdout_changes_nothing`, `test_forged_json_on_stderr_and_closed_descriptors` | pass |
| 15 | Hidden test case leak | A student sees one of the two test cases (the hidden one is not listed at all); the student report has no `actual_output`, `error_message`, `input` or `expected_output` on the hidden result; the UAT driver found 0 leaks over 10 students; the lecturer of the Course does see the hidden case. `expected_output` never enters the sandbox by design (the sandbox gets one command and stdin) | pass |
| 16 | Leak through `/proc/<pid>/cmdline` | A probe running as the backend's own user scanned every readable `/proc/*/cmdline` and `environ` thousands of times while a submission with unique markers in a hidden test case was graded: the markers were never seen, while a control process with a marker in its argv was found. A reader running as root was not tested | pass |
| 17 | IDOR | Another student reading a submission, its report and its code: `404`; regrade and cancel of another's submission: `404`; the lecturer of another Course reading the code or the recap: `404` | pass |
| 18 | Cross-class access | A student of Course B: `404` on the assignment, on its test cases, and on an upload to it | pass |
| 19 | Role escalation | A student promoting themself to `ADMIN`: `403`, role unchanged, and an audit row with the student's `user_id`; a lecturer cannot change roles, a student cannot list users, a lecturer cannot read the audit log; registering with `role: ADMIN` is refused (`400`) | pass |
| 20 | Session after logout | Refresh token after logout: `401`; after a password change both the token from before and the older one: `401` | pass |
| 21 | Result instability | `Determinism.test_set_ordering_is_stable_across_runs` (20 runs), `test_time_zone_and_locale`, `test_isolated_mode_would_defeat_the_fixed_hash_seed` | pass |
| 22 | Leftovers after a crash | `Supervision.test_supervisor_sigkill_leaves_nothing_running_and_sweep_cleans_up`, `test_sigterm_ends_the_run_and_cleans_up`; the whole-backend case is in the next section | pass |

Result: 21 pass, 1 partial (13), 0 not run.

## Failure injection (NFR-04, SRS 11.2, WBS 6.3)

Procedure, on the live local backend: upload a program that sleeps 8 s, wait until the submission is
`RUNNING`, `SIGKILL` the backend and every `sandbox-exec`, start the backend again, and watch the
submission and the leftovers.

- Submission 29: `RUNNING` when killed. The backend was back after 2.5 s. At start-up the sweep ran
  (`swept leftovers of dead sandbox runs`), the submission was requeued (`"reason":"reconcile"`), and
  it was `COMPLETED` with 10 of 10 about 8 s later. No `sbx-*` cgroup directory and no sandbox process
  was left, right after the kill or after the recovery.
- An earlier submission (28), `RUNNING` when a first attempt of the same probe stopped the backend,
  was requeued the same way when the backend was started again, and graded 10 of 10.

What this does not show: each backend start here gets a new delegated cgroup scope, so the old one
disappeared with the old process and the sweep had no cgroups or mounts to remove. The case the sweep
exists for, a restarted unit that keeps its cgroup and finds `sbx-*` directories and `tmpfs` mounts
from the dead run, needs the real unit. To run it on the campus server (`Restart=on-failure` brings
the unit back by itself after 2 s):

```sh
ADMIN_EMAIL=... ADMIN_PASSWORD=... python3 infra/qa/crash_probe.py \
    --kill 'sudo systemctl kill -s SIGKILL autograder; sudo pkill -9 -f sandbox-exec' \
    --cgroup-glob '/sys/fs/cgroup/system.slice/autograder.service/sbx-*'
sudo journalctl -u autograder --since '5 min ago' --no-pager -o cat | grep -E 'swept|reconcile|graded'
```

[`infra/qa/crash_probe.py`](../../infra/qa/crash_probe.py) does the procedure above through the API
and exits 1 if the submission does not finish or anything is left. Run on the local backend with its
`--start` option, it printed the same result as the manual run (a new submission, `COMPLETED` 10 of
10 about 7 s after the backend answered again, nothing left).

## Gaps

1. **Fairness under load (scenario 13)** and the NFR-01 load tests need k6 and staging or a quiet
   server; they are Robertus's WBS 6.2.
2. **The campus server** runs a build from before the submission API, so none of the API checks have
   run there. After the deploy, run `infra/qa/adversarial_api.py` and `infra/qa/cmdline_probe.py`
   there, on a database without real data, and the crash test above, and add their results here.
3. **The ASan/UBSan variant** was not run locally; CI covers it.
4. **Scope of scenario 16**: the reader was the backend's own user. root can read anything on the
   host, which no process-level measure here prevents.
5. **The syscall allowlist** itself was not audited syscall by syscall in this report; the suite
   tests the denied calls it lists.

## How to repeat

```sh
sandbox/tests/run.sh /usr/local/bin/sandbox-exec        # the suite, on a host with cgroup v2
ADMIN_EMAIL=... ADMIN_PASSWORD=... python3 infra/qa/adversarial_api.py --base http://127.0.0.1:8080
ADMIN_EMAIL=... ADMIN_PASSWORD=... python3 infra/qa/cmdline_probe.py --base http://127.0.0.1:8080
```

The three scripts create users, Courses and rejected uploads; run them only on a database without
real data, with the administrator of that database. `infra/qa/clean.sql` removes what they made
(`sudo -u postgres psql -d autograder -v ON_ERROR_STOP=1 -f infra/qa/clean.sql`); on the local
database it removed 38 test users with their Courses, submissions and audit rows and left the 11 UAT
users and their submission untouched.
