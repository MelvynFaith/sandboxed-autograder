# Load test

The load tests of SRS Bagian 11.4. [k6](https://k6.io) measures the end-to-end time of a submission:
from the start of `POST /api/v1/submissions` to the final WebSocket frame, not the HTTP response,
which only says the submission is queued.

| Script | Scenario | Requirement | Passes when |
|---|---|---|---|
| `submit.js` | 5, 10 and 30 accounts submit at once | NFR-01a | every submission `COMPLETED`, average at most 10 s |
| `submit.js` | 60 and 100 accounts submit at once | NFR-01b | every submission `COMPLETED`; the latency is a baseline to report |
| `ratelimit.js` | one account sends 200 submissions in a row while 10 others submit | FR-14 | one of the 200 accepted, the rest refused with `429` and a reason; the 10 graded within 10 s on average |
| `fairness.js` | one account floods the queue, then 10 others submit | FR-13 | the 10 average at most twice their time without the flood |

**Status:** needs the upload endpoint, `POST /api/v1/submissions`, which is not written yet; until
then every submission is refused with `404`. The scripts were run against a stand-in for that
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
`LOAD_PASSWORD` and enrolls them in course `LOAD`, which has the assignment every account submits
to and 30 flood assignments for `fairness.js`. Each has the five test cases of the model in SRS
Bagian 5.1, and every account submits the same program, which scores 100. The scripts find the
course and its assignments through the API.

```sh
export LOAD_PASSWORD=$(openssl rand -base64 18)
sudo -v && infra/loadtest/seed.sh
```

Then run each scenario, followed by `report` and `reset`:

```sh
k6 run -e ACCOUNTS=30 infra/loadtest/submit.js     # likewise 5, 10, 60 and 100
k6 run infra/loadtest/ratelimit.js
k6 run -e BASELINE_MS=AVG infra/loadtest/fairness.js
infra/loadtest/seed.sh report
infra/loadtest/seed.sh reset
```

- `k6 run` exits `0` only if the scenario passes. Its summary has `e2e_duration` (average, median,
  p95, maximum) and `completed`, the share of submissions that ended `COMPLETED` within `WAIT_S`
  (default 300 s). Each account logs in first, in `setup`, so logins are not part of the
  measurement.
- `ratelimit.js` also prints each refusal reason with its count, and `spam_accepted` and
  `spam_refused_with_reason`.
- `fairness.js` needs the baseline: `AVG` is the `e2e_duration` average, in milliseconds, of the
  10-account run of `submit.js`. FR-14 lets the flooding account queue one submission per
  assignment, so it floods with one to each flood assignment, all sent just before the 10 others
  submit. With the queue serving students in turn, the flood gets one turn per round. A queue that
  served submissions in order of arrival would make the 10 wait for all 30 and fail.
- `report` waits until nothing is queued or running (`fairness.js` ends while its flood is still
  being graded). Then it reads the database: how many submissions ended in which status, the
  server's own time from `submitted_at` to `graded_at`, and whether every one scored 100/100. It
  exits `0` only then.
- `reset` deletes the submissions and their stored files, so the next run starts from an empty
  queue and is not refused by FR-14's 30 seconds between submissions or 30 per assignment.

When done, `infra/loadtest/seed.sh clean` removes the accounts, the course and everything under it.
The password is read from the environment by every script and never appears on a command line; it
must be 12 characters or more and contain no `"` or `\`. Each script's header lists its options.

### Server CPU and memory

SRS Bagian 11.4 asks for the server's CPU and memory use at the typical levels. The backend's cgroup
counts the sandboxes too:

```sh
cg=/sys/fs/cgroup$(systemctl show -P ControlGroup autograder)
grep usage_usec "$cg/cpu.stat"
cat "$cg/memory.peak"
```

Read `usage_usec` before and after a run: the difference is the CPU time it took, in microseconds.
`memory.peak` is the highest use since the service started, so restart the service before a run to
measure that run alone.

## Files

| File | What it is |
|---|---|
| `seed.sh` | Accounts, course and assignments; `report`, `reset` and `clean` |
| `lib.js` | Shared by the scripts: options, logging in, finding the assignments, one submission followed to its final frame |
| `submit.js` | NFR-01a and NFR-01b: one submission per account, all at once |
| `ratelimit.js` | FR-14: 200 submissions in a row from one account, beside 10 others |
| `fairness.js` | FR-13: 10 submissions behind one account's flood, against the baseline |
