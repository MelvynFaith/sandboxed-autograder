# Grading Harness

A Python program that grades one submission. For each test case it calls `sandbox-exec` once,
sends the test input on stdin, compares the actual output with the expected output, and assigns a
verdict. It returns a single JSON report to the calling worker and **never opens a database
connection.**

**Owner:** Robertus Geraldyn Alexandro Gabeler
**Status:** not started. Sprint 2 target: integration with the sandbox.

## Responsibilities

- Read the assignment definition (test cases, resource limits, match modes) from the file path the
  worker passes in.
- Run test cases sequentially. Never run student code outside `sandbox-exec`.
- Compare output with `EXACT` or `WHITESPACE_INSENSITIVE` matching.
- Map sandbox outcomes to verdicts: `PASSED`, `FAILED`, `TIMEOUT`, `MEMORY_LIMIT_EXCEEDED`,
  `RUNTIME_ERROR`, `SYNTAX_ERROR`, `SECURITY_VIOLATION`.
- Compute weighted per-test scores and the total score.

## Build and test

To be added with the first implementation. CI activates the `grader` job once `pyproject.toml`
exists here. The job installs the package, runs `ruff check`, and runs `pytest`.
