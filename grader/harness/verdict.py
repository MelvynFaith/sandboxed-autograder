"""Map a sandbox-exec report to a test case verdict (SRS 3.2.1.7)."""

from collections.abc import Mapping
from enum import Enum
from typing import Any

from harness.compare import MatchMode, outputs_match


class Verdict(str, Enum):
    """Result.status values. Names must stay identical in Python, YAML and SQL."""

    PASSED = "PASSED"
    FAILED = "FAILED"
    TIMEOUT = "TIMEOUT"
    MEMORY_LIMIT_EXCEEDED = "MEMORY_LIMIT_EXCEEDED"
    RUNTIME_ERROR = "RUNTIME_ERROR"
    SYNTAX_ERROR = "SYNTAX_ERROR"
    SECURITY_VIOLATION = "SECURITY_VIOLATION"


class InternalError(Exception):
    """The report cannot be graded. This is not a test case status.

    The Harness must stop without a report, so the worker marks the submission FAILED.
    """


_SYNTAX_ERROR_PREFIXES = ("SyntaxError", "IndentationError", "TabError")
_MEMORY_ERROR_PREFIX = "MemoryError"

# termination_reason values that map to a verdict without looking at the output.
_DIRECT = {
    "SIGNALED": Verdict.RUNTIME_ERROR,
    "TIMEOUT_WALL": Verdict.TIMEOUT,
    "TIMEOUT_CPU": Verdict.TIMEOUT,
    "MEMORY_LIMIT": Verdict.MEMORY_LIMIT_EXCEEDED,
    "OUTPUT_LIMIT": Verdict.FAILED,
    "SECURITY_VIOLATION": Verdict.SECURITY_VIOLATION,
}


def _last_line(text: str) -> str:
    lines = text.strip().splitlines()
    return lines[-1].strip() if lines else ""


def _classify_failed_exit(stderr: str) -> Verdict:
    """Classify an EXITED run with a non-zero exit code by the last stderr line."""
    last = _last_line(stderr)
    if last.startswith(_MEMORY_ERROR_PREFIX):
        return Verdict.MEMORY_LIMIT_EXCEEDED
    if last.startswith(_SYNTAX_ERROR_PREFIXES):
        return Verdict.SYNTAX_ERROR
    return Verdict.RUNTIME_ERROR


def classify(report: Mapping[str, Any], expected_output: str, mode: MatchMode) -> Verdict:
    """Return the verdict of one test case from its sandbox-exec report.

    Raises InternalError for INTERNAL_ERROR, an unknown termination_reason, or a
    malformed report. Truncated output never passes, even if its prefix matches.
    """
    reason = report.get("termination_reason")
    if reason == "INTERNAL_ERROR":
        raise InternalError(str(report.get("error") or "sandbox-exec reported INTERNAL_ERROR"))
    if reason in _DIRECT:
        return _DIRECT[reason]
    if reason != "EXITED":
        raise InternalError(f"unknown termination_reason: {reason!r}")

    exit_code = report.get("exit_code")
    if not isinstance(exit_code, int):
        raise InternalError("EXITED report without an integer exit_code")
    if exit_code != 0:
        return _classify_failed_exit(str(report.get("stderr") or ""))

    if report.get("output_truncated"):
        return Verdict.FAILED
    stdout = report.get("stdout")
    if not isinstance(stdout, str):
        raise InternalError("stdout in the report is not a string")
    return Verdict.PASSED if outputs_match(stdout, expected_output, mode) else Verdict.FAILED
