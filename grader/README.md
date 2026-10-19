# Grading Harness

A Python program that grades one submission. For each test case it calls `sandbox-exec` once,
sends the test input on stdin, compares the actual output with the expected output, and assigns a
verdict. It returns a single JSON report to the calling worker and **never opens a database
connection.**

**Owner:** Robertus Geraldyn Alexandro Gabeler
**Status:** implemented and tested against the real `sandbox-exec`. Requirements: FR-07 (comparison,
status mapping, scoring) and WBS 4.2 (integration with the sandbox).

## Responsibilities

- Read the assignment definition (test cases, resource limits, match modes) from the file path the
  worker passes in.
- Run test cases sequentially. Never run student code outside `sandbox-exec`.
- Compare output with `EXACT` or `WHITESPACE_INSENSITIVE` matching.
- Map sandbox outcomes to verdicts: `PASSED`, `FAILED`, `TIMEOUT`, `MEMORY_LIMIT_EXCEEDED`,
  `RUNTIME_ERROR`, `SYNTAX_ERROR`, `SECURITY_VIOLATION`.
- Compute weighted per-test scores and the total score.

## Layout

| File | Purpose |
|---|---|
| `harness/compare.py` | `MatchMode` and `outputs_match()` |
| `harness/verdict.py` | `Verdict`, `classify()` (sandbox report to verdict), `InternalError` |
| `harness/runner.py` | Builds the `sandbox-exec` command and runs one test case |
| `harness/grade.py` | Reads the definition file, runs every test case, builds the report |
| `harness/__main__.py` | Command line entry point |

## Install

From this directory, with Python 3.11 or newer:

    python3 -m venv .venv
    source .venv/bin/activate
    pip install -e ".[dev]"

In production, the interpreter named in the worker's `HARNESS_COMMAND` must have the package
installed (`pip install .`), for example `HARNESS_COMMAND="/opt/grader/.venv/bin/python -m harness"`.

## Run

The worker starts the Harness like this:

    python -m harness --definition PATH --code PATH

- `--definition` is a JSON file with the assignment definition (`schema_version` 1, `submission_id`,
  `resource_limit`, and `test_cases` with `id`, `external_id`, `input`, `expected_output`, `weight`,
  `match_mode`). The exact schema is in `scheduler/internal/harness/harness.go`.
- `--code` is the student's source file.

The environment of the Harness is exactly what the worker sets. Two variables are required:

| Variable | Meaning |
|---|---|
| `SANDBOX_EXEC` | Absolute path of `sandbox-exec` |
| `SANDBOX_CGROUP_ROOT` | Delegated cgroup passed to `sandbox-exec` as `--cgroup-root` |

On success the Harness writes one JSON document to standard output (SRS Appendix C) and exits 0:
`submission_id`, `total_score`, `max_score`, and one result per test case. `error_message` and
`violation` are present only when there is one. On any failure it writes nothing to standard output,
prints the reason to standard error, and exits 1, so the worker marks the submission `FAILED`.

## Verdicts

| `sandbox-exec` outcome | Verdict |
|---|---|
| `EXITED`, exit code 0, output matches | `PASSED` |
| `EXITED`, exit code 0, output differs or is truncated | `FAILED` |
| `EXITED`, exit code not 0, last stderr line starts with `MemoryError` | `MEMORY_LIMIT_EXCEEDED` |
| `EXITED`, exit code not 0, last stderr line starts with `SyntaxError`, `IndentationError` or `TabError` | `SYNTAX_ERROR` |
| `EXITED`, exit code not 0, anything else | `RUNTIME_ERROR` |
| `SIGNALED` | `RUNTIME_ERROR` |
| `TIMEOUT_WALL`, `TIMEOUT_CPU` | `TIMEOUT` |
| `MEMORY_LIMIT` | `MEMORY_LIMIT_EXCEEDED` |
| `OUTPUT_LIMIT` | `FAILED` (`is_truncated` is true) |
| `SECURITY_VIOLATION` | `SECURITY_VIOLATION` (with the denied syscall number) |
| `INTERNAL_ERROR`, unknown reason, or a malformed report | not a verdict: the Harness stops without a report |

A test case scores its full weight when it is `PASSED` and 0 otherwise. A truncated output never
passes, even if its prefix matches the expected output.

`WHITESPACE_INSENSITIVE` compares the outputs after `str.split()`, which splits on any run of Unicode
whitespace and ignores leading and trailing whitespace. Tokens are never merged: `12` does not match
`1 2`.

## Build and test

Run from this directory, with the virtual environment active:

    ruff check .
    pytest

The unit tests mock `subprocess`, so they need no sandbox. To try the Harness against the real
`sandbox-exec`, build it (`make` in `sandbox/`) and run the command inside a delegated cgroup with
`sandbox/tests/run.sh`, which exports `SBX_BIN` and `SBX_CGROUP_ROOT`:

    sandbox/tests/run.sh "$PWD/sandbox/sandbox-exec" --exec ./your-script.sh

Inside the script, set `SANDBOX_EXEC="$SBX_BIN"` and `SANDBOX_CGROUP_ROOT="$SBX_CGROUP_ROOT"`, then
call `python -m harness`. See `sandbox/README.md` ("cgroup delegation") for the host requirements.
