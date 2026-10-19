# Load test

The load test of SRS Bagian 11.4. [k6](https://k6.io) measures the end-to-end time of a submission:
from the start of `POST /api/v1/submissions` to the final WebSocket frame, not the HTTP response,
which only says the submission is queued. It runs both load levels of the SRS:

| Level | Accounts submitting at once | Requirement | Threshold in `submit.js` |
|---|---|---|---|
| Typical | 5, 10, 30 | NFR-01a | every submission `COMPLETED`, average at most 10 s |
| Stress ceiling | 60, 100 | NFR-01b | every submission `COMPLETED`; the latency is a baseline to report |

**Status:** needs the upload endpoint, `POST /api/v1/submissions`, which is not written yet; until
then every submission is refused with `404`. Both scripts were run against a stand-in for that
endpoint that follows the contract in SRS Lampiran B (multipart `assignment_id` and `file`, `202`
with the submission `id`).

## Where to run it

On staging (SRS Bagian 13.1), not on a server with real data: it adds 100 student accounts and
hundreds of submissions. `seed.sh clean` removes exactly what it added. The backend must be installed
as in [infra/README.md](../README.md). k6 v2.3 was used; it runs on any machine that reaches the API.
Run on the server itself, it competes with the backend for CPU, which lengthens the times; its own
use is outside the backend's cgroup, so the CPU figures below still count the backend alone.

## Running it

FR-14 lets a student have one submission in the queue per assignment, so N submissions at once need
N accounts (SRS Bagian 11.4). `seed.sh` registers them through the API with the password in
`LOAD_PASSWORD`, enrolls them in one course, and creates one assignment with the five test cases of
the model in SRS Bagian 5.1. Every account submits the same program, which scores 100.

```sh
export LOAD_PASSWORD=$(openssl rand -base64 18)
sudo -v && infra/loadtest/seed.sh
```

It prints the assignment's id. Then, for each level:

```sh
k6 run -e ACCOUNTS=30 -e ASSIGNMENT=ID infra/loadtest/submit.js
infra/loadtest/seed.sh report
infra/loadtest/seed.sh reset
```

- `k6 run` exits `0` only if the thresholds hold. Its summary has `e2e_duration` (average, median,
  p95, maximum) and `completed`, the share of submissions that ended `COMPLETED` within `WAIT_S`
  (default 300 s). Each account logs in first, in `setup`, so logins are not part of the measurement.
- `report` reads the database: how many submissions ended in which status, the server's own time from
  `submitted_at` to `graded_at`, and whether every one scored 100/100. It exits `0` only then.
- `reset` deletes the submissions and their stored files, so the next level starts from an empty
  queue and is not refused by FR-14's 30 seconds between submissions or 30 per assignment.

When done, `infra/loadtest/seed.sh clean` removes the accounts, the course and everything under them.
The password is read from the environment by both scripts and never appears on a command line; it
must be 12 characters or more and contain no `"` or `\`.

### Server CPU and memory

SRS Bagian 11.4 asks for the server's CPU and memory use at the typical levels. The backend's cgroup
counts the sandboxes too:

```sh
cg=/sys/fs/cgroup$(systemctl show -P ControlGroup autograder)
grep usage_usec "$cg/cpu.stat"
cat "$cg/memory.peak"
```

Read `usage_usec` before and after a level: the difference is the CPU time it took, in
microseconds. `memory.peak` is the highest use since the service started, so restart the service
before a level to measure that level alone.

### Not covered

The other two scenarios of SRS Bagian 11.4: one account sending 200 submissions in a row (FR-14),
and the fairness test of one account flooding the queue while ten others submit (FR-13).

## Files

| File | What it is |
|---|---|
| `seed.sh` | Accounts, course and assignment; `report`, `reset` and `clean` |
| `submit.js` | The k6 script: one submission per account, all at once, each followed on the WebSocket |
