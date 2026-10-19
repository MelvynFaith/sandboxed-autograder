import json
import subprocess

import pytest

from harness import runner
from harness.compare import MatchMode
from harness.runner import Limits, SandboxConfig, build_command, run_test_case
from harness.verdict import InternalError, Verdict

CONFIG = SandboxConfig(sandbox_exec="/opt/sandbox-exec", cgroup_root="/sys/fs/cgroup/grader")
LIMITS = Limits(
    cpu_time_ms=2000,
    memory_mb=64,
    max_processes=5,
    timeout_s=5,
    max_output_bytes=1048576,
    tmpfs_size_mb=32,
)


def make_report(**overrides):
    report = {
        "schema_version": 1,
        "termination_reason": "EXITED",
        "exit_code": 0,
        "signal": None,
        "wall_ms": 120,
        "cpu_ms": 80,
        "memory_kb": 9000,
        "memory_source": "CGROUP_PEAK",
        "stdout": "120\n",
        "stderr": "",
        "output_truncated": False,
        "violation": None,
        "pids_limit_hit": False,
        "error": None,
    }
    report.update(overrides)
    return report


def install_fake(monkeypatch, report=None, returncode=0, stdout=None, raises=None):
    calls = []

    def fake_run(command, **kwargs):
        calls.append((command, kwargs))
        if raises is not None:
            raise raises
        out = stdout if stdout is not None else json.dumps(report).encode()
        return subprocess.CompletedProcess(command, returncode, stdout=out, stderr=b"")

    monkeypatch.setattr(runner.subprocess, "run", fake_run)
    return calls


def run(expected="120\n", mode=MatchMode.EXACT, input_text="5\n"):
    return run_test_case(CONFIG, LIMITS, "/tmp/solution.py", input_text, expected, mode)


def test_build_command_matches_the_man_page_example():
    command = build_command(CONFIG, LIMITS, "/tmp/solution.py")
    assert command == [
        "/opt/sandbox-exec",
        "--cgroup-root", "/sys/fs/cgroup/grader",
        "--cpu-time-ms", "2000",
        "--timeout-s", "5",
        "--memory-mb", "64",
        "--max-processes", "5",
        "--max-output-bytes", "1048576",
        "--tmpfs-size-mb", "32",
        "--copy", "/tmp/solution.py:main.py",
        "--",
        "/usr/bin/python3", "-s", "-P", "-B", "main.py",
    ]  # fmt: skip


def test_python_is_never_started_with_dash_i():
    assert "-I" not in build_command(CONFIG, LIMITS, "/tmp/solution.py")


def test_config_from_env():
    env = {"SANDBOX_EXEC": "/a", "SANDBOX_CGROUP_ROOT": "/b"}
    assert SandboxConfig.from_env(env) == SandboxConfig("/a", "/b")


def test_config_from_env_missing_variable_raises():
    with pytest.raises(InternalError):
        SandboxConfig.from_env({"SANDBOX_EXEC": "/a"})


def test_limits_from_dict():
    data = {
        "cpu_time_ms": 2000,
        "memory_mb": 64,
        "max_processes": 5,
        "timeout_s": 5,
        "max_output_bytes": 1048576,
        "tmpfs_size_mb": 32,
    }
    assert Limits.from_dict(data) == LIMITS


@pytest.mark.parametrize("bad", [None, 0, -1, "5", True, 1.5])
def test_limits_from_dict_rejects_bad_values(bad):
    data = {
        "cpu_time_ms": 2000,
        "memory_mb": bad,
        "max_processes": 5,
        "timeout_s": 5,
        "max_output_bytes": 1048576,
        "tmpfs_size_mb": 32,
    }
    with pytest.raises(InternalError):
        Limits.from_dict(data)


def test_input_goes_to_stdin_and_expected_output_stays_out(monkeypatch):
    calls = install_fake(monkeypatch, make_report())
    run(expected="SECRET-EXPECTED\n", input_text="5\n")
    command, kwargs = calls[0]
    assert kwargs["input"] == b"5\n"
    assert kwargs["timeout"] == LIMITS.timeout_s + runner.KILL_GRACE_S
    assert not any("SECRET-EXPECTED" in part for part in command)


def test_passed_result_fields(monkeypatch):
    install_fake(monkeypatch, make_report())
    result = run()
    assert result.status is Verdict.PASSED
    assert result.actual_output == "120\n"
    assert result.is_truncated is False
    assert result.exec_time_ms == 120
    assert result.memory_kb == 9000
    assert result.memory_source == "CGROUP_PEAK"
    assert result.error_message is None
    assert result.syscall_nr is None


def test_wrong_output_fails(monkeypatch):
    install_fake(monkeypatch, make_report(stdout="121\n"))
    assert run().status is Verdict.FAILED


def test_runtime_error_carries_last_stderr_line(monkeypatch):
    stderr = "Traceback (most recent call last):\nValueError: bad input\n"
    install_fake(monkeypatch, make_report(exit_code=1, stderr=stderr))
    result = run()
    assert result.status is Verdict.RUNTIME_ERROR
    assert result.error_message == "ValueError: bad input"


def test_syntax_error_carries_message(monkeypatch):
    install_fake(monkeypatch, make_report(exit_code=1, stderr="SyntaxError: invalid syntax\n"))
    result = run()
    assert result.status is Verdict.SYNTAX_ERROR
    assert result.error_message == "SyntaxError: invalid syntax"


def test_timeout(monkeypatch):
    install_fake(monkeypatch, make_report(termination_reason="TIMEOUT_WALL", exit_code=None))
    assert run().status is Verdict.TIMEOUT


def test_security_violation_keeps_syscall_number(monkeypatch):
    report = make_report(
        termination_reason="SECURITY_VIOLATION", exit_code=None, violation={"syscall_nr": 101}
    )
    install_fake(monkeypatch, report)
    result = run()
    assert result.status is Verdict.SECURITY_VIOLATION
    assert result.syscall_nr == 101


def test_truncated_output_is_reported_and_fails(monkeypatch):
    report = make_report(termination_reason="OUTPUT_LIMIT", exit_code=None, output_truncated=True)
    install_fake(monkeypatch, report)
    result = run()
    assert result.status is Verdict.FAILED
    assert result.is_truncated is True


def test_nonzero_sandbox_exit_status_raises(monkeypatch):
    install_fake(monkeypatch, make_report(), returncode=70)
    with pytest.raises(InternalError):
        run()


def test_invalid_json_raises(monkeypatch):
    install_fake(monkeypatch, stdout=b"not json")
    with pytest.raises(InternalError):
        run()


def test_report_that_is_not_an_object_raises(monkeypatch):
    install_fake(monkeypatch, stdout=b"[1, 2]")
    with pytest.raises(InternalError):
        run()


def test_internal_error_report_raises(monkeypatch):
    report = make_report(termination_reason="INTERNAL_ERROR", error="cgroup setup failed")
    install_fake(monkeypatch, report)
    with pytest.raises(InternalError):
        run()


def test_unknown_memory_source_raises(monkeypatch):
    install_fake(monkeypatch, make_report(memory_source="GUESS"))
    with pytest.raises(InternalError):
        run()


def test_missing_sandbox_binary_raises(monkeypatch):
    install_fake(monkeypatch, raises=FileNotFoundError("/opt/sandbox-exec"))
    with pytest.raises(InternalError):
        run()


def test_hung_sandbox_raises(monkeypatch):
    install_fake(monkeypatch, raises=subprocess.TimeoutExpired("sandbox-exec", 15))
    with pytest.raises(InternalError):
        run()


def test_no_descriptor_is_passed_on_to_sandbox_exec(monkeypatch):
    # HARNESS_PROGRESS_FD must not reach the sandbox: subprocess closes it by default.
    calls = install_fake(monkeypatch, make_report())
    run()
    kwargs = calls[0][1]
    assert kwargs.get("close_fds", True) is True
    assert not kwargs.get("pass_fds")
