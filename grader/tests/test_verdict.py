import pytest

from harness.compare import MatchMode
from harness.verdict import InternalError, Verdict, classify

TRACEBACK = (
    "Traceback (most recent call last):\n"
    '  File "main.py", line 1, in <module>\n'
    "{last}\n"
)


def make_report(**overrides):
    report = {
        "schema_version": 1,
        "termination_reason": "EXITED",
        "exit_code": 0,
        "signal": None,
        "stdout": "120\n",
        "stderr": "",
        "output_truncated": False,
        "violation": None,
        "pids_limit_hit": False,
        "error": None,
    }
    report.update(overrides)
    return report


def run(report, expected="120\n", mode=MatchMode.EXACT):
    return classify(report, expected, mode)


def test_verdict_names_match_the_srs_enum():
    assert {v.value for v in Verdict} == {
        "PASSED",
        "FAILED",
        "TIMEOUT",
        "MEMORY_LIMIT_EXCEEDED",
        "RUNTIME_ERROR",
        "SYNTAX_ERROR",
        "SECURITY_VIOLATION",
    }


def test_exit_zero_matching_output_passes():
    assert run(make_report()) is Verdict.PASSED


def test_exit_zero_different_output_fails():
    assert run(make_report(stdout="121\n")) is Verdict.FAILED


def test_whitespace_insensitive_mode_is_used():
    report = make_report(stdout="120")
    assert run(report, mode=MatchMode.WHITESPACE_INSENSITIVE) is Verdict.PASSED
    assert run(report, mode=MatchMode.EXACT) is Verdict.FAILED


@pytest.mark.parametrize(
    ("reason", "expected"),
    [
        ("SIGNALED", Verdict.RUNTIME_ERROR),
        ("TIMEOUT_WALL", Verdict.TIMEOUT),
        ("TIMEOUT_CPU", Verdict.TIMEOUT),
        ("MEMORY_LIMIT", Verdict.MEMORY_LIMIT_EXCEEDED),
        ("OUTPUT_LIMIT", Verdict.FAILED),
        ("SECURITY_VIOLATION", Verdict.SECURITY_VIOLATION),
    ],
)
def test_direct_termination_reasons(reason, expected):
    report = make_report(termination_reason=reason, exit_code=None, stdout="120\n")
    assert run(report) is expected


def test_output_limit_never_passes_even_if_stdout_matches():
    report = make_report(termination_reason="OUTPUT_LIMIT", exit_code=None, stdout="120\n")
    assert run(report) is Verdict.FAILED


def test_truncated_output_with_exit_zero_never_passes():
    report = make_report(output_truncated=True, stdout="120\n")
    assert run(report, mode=MatchMode.WHITESPACE_INSENSITIVE) is Verdict.FAILED


@pytest.mark.parametrize(
    "last",
    [
        "SyntaxError: invalid syntax",
        "IndentationError: unexpected indent",
        "TabError: inconsistent use of tabs and spaces in indentation",
    ],
)
def test_syntax_errors(last):
    report = make_report(exit_code=1, stderr=TRACEBACK.format(last=last))
    assert run(report) is Verdict.SYNTAX_ERROR


def test_memory_error_on_last_stderr_line():
    report = make_report(exit_code=1, stderr=TRACEBACK.format(last="MemoryError"))
    assert run(report) is Verdict.MEMORY_LIMIT_EXCEEDED


def test_other_exception_is_runtime_error():
    report = make_report(exit_code=1, stderr=TRACEBACK.format(last="ValueError: bad input"))
    assert run(report) is Verdict.RUNTIME_ERROR


def test_syntax_error_text_before_the_last_line_is_runtime_error():
    stderr = "SyntaxError: printed by the student\nValueError: bad\n"
    assert run(make_report(exit_code=1, stderr=stderr)) is Verdict.RUNTIME_ERROR


def test_nonzero_exit_with_empty_stderr_is_runtime_error():
    assert run(make_report(exit_code=2, stderr="")) is Verdict.RUNTIME_ERROR


def test_internal_error_raises():
    report = make_report(termination_reason="INTERNAL_ERROR", error="cgroup setup failed")
    with pytest.raises(InternalError):
        run(report)


def test_unknown_termination_reason_raises():
    with pytest.raises(InternalError):
        run(make_report(termination_reason="BANANA"))


def test_exited_without_exit_code_raises():
    with pytest.raises(InternalError):
        run(make_report(exit_code=None))


def test_bytes_stdout_raises():
    with pytest.raises(InternalError):
        run(make_report(stdout=b"120\n"))
