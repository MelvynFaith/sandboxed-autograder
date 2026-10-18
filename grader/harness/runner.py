"""Run one test case through sandbox-exec and turn its report into a test case result."""

import json
import subprocess
from collections.abc import Mapping
from dataclasses import dataclass, fields
from typing import Any

from harness.compare import MatchMode
from harness.verdict import InternalError, Verdict, classify

# Environment variables the worker passes to the Harness (the Harness environment is exactly
# what the deployment sets, nothing is inherited).
ENV_SANDBOX_EXEC = "SANDBOX_EXEC"
ENV_CGROUP_ROOT = "SANDBOX_CGROUP_ROOT"

SOLUTION_NAME = "main.py"
# sandbox-exec man page: -I implies -E, which ignores PYTHONHASHSEED. Use -s -P -B.
PYTHON_ARGV = ("/usr/bin/python3", "-s", "-P", "-B", SOLUTION_NAME)
# Seconds added to timeout_s before the Harness gives up on sandbox-exec itself.
KILL_GRACE_S = 10
MEMORY_SOURCES = ("CGROUP_PEAK", "CGROUP_SAMPLED", "RUSAGE")
ERROR_MESSAGE_MAX = 500


@dataclass(frozen=True)
class SandboxConfig:
    """Where to find sandbox-exec and the delegated cgroup."""

    sandbox_exec: str
    cgroup_root: str

    @classmethod
    def from_env(cls, env: Mapping[str, str]) -> "SandboxConfig":
        try:
            return cls(sandbox_exec=env[ENV_SANDBOX_EXEC], cgroup_root=env[ENV_CGROUP_ROOT])
        except KeyError as exc:
            raise InternalError(f"environment variable {exc.args[0]} is not set") from exc


@dataclass(frozen=True)
class Limits:
    """The assignment's resource limits (the resource_limit object of the definition)."""

    cpu_time_ms: int
    memory_mb: int
    max_processes: int
    timeout_s: int
    max_output_bytes: int
    tmpfs_size_mb: int

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> "Limits":
        values = {}
        for field in fields(cls):
            value = data.get(field.name)
            if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
                raise InternalError(f"resource_limit.{field.name} must be a positive integer")
            values[field.name] = value
        return cls(**values)


@dataclass(frozen=True)
class TestCaseResult:
    """Everything the final report needs about one test case, except weight and score."""

    __test__ = False  # not a pytest test class

    status: Verdict
    actual_output: str
    is_truncated: bool
    exec_time_ms: int
    memory_kb: int
    memory_source: str
    error_message: str | None = None
    syscall_nr: int | None = None


def build_command(config: SandboxConfig, limits: Limits, code_path: str) -> list[str]:
    """Return the sandbox-exec command line for one run of the student's code."""
    return [
        config.sandbox_exec,
        "--cgroup-root", config.cgroup_root,
        "--cpu-time-ms", str(limits.cpu_time_ms),
        "--timeout-s", str(limits.timeout_s),
        "--memory-mb", str(limits.memory_mb),
        "--max-processes", str(limits.max_processes),
        "--max-output-bytes", str(limits.max_output_bytes),
        "--tmpfs-size-mb", str(limits.tmpfs_size_mb),
        "--copy", f"{code_path}:{SOLUTION_NAME}",
        "--",
        *PYTHON_ARGV,
    ]  # fmt: skip


def _non_negative_int(report: Mapping[str, Any], key: str) -> int:
    value = report.get(key)
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        return 0
    return value


def _last_stderr_line(report: Mapping[str, Any]) -> str:
    lines = str(report.get("stderr") or "").strip().splitlines()
    return lines[-1].strip()[:ERROR_MESSAGE_MAX] if lines else ""


def _syscall_nr(report: Mapping[str, Any]) -> int | None:
    violation = report.get("violation")
    if isinstance(violation, dict):
        nr = violation.get("syscall_nr")
        if isinstance(nr, int) and not isinstance(nr, bool) and nr >= 0:
            return nr
    return None


def run_test_case(
    config: SandboxConfig,
    limits: Limits,
    code_path: str,
    input_text: str,
    expected_output: str,
    mode: MatchMode,
) -> TestCaseResult:
    """Run the student's code once with input_text on stdin and classify the outcome.

    Raises InternalError if sandbox-exec cannot run, exits with a non-zero status, or does not
    return a usable report. The expected output never goes into the sandbox.
    """
    command = build_command(config, limits, code_path)
    try:
        proc = subprocess.run(
            command,
            input=input_text.encode("utf-8", errors="replace"),
            capture_output=True,
            timeout=limits.timeout_s + KILL_GRACE_S,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise InternalError(f"cannot run sandbox-exec: {exc}") from exc
    if proc.returncode != 0:
        raise InternalError(f"sandbox-exec exited with status {proc.returncode}")

    try:
        report = json.loads(proc.stdout.decode("utf-8", errors="replace"))
    except ValueError as exc:
        raise InternalError(f"sandbox-exec report is not valid JSON: {exc}") from exc
    if not isinstance(report, dict):
        raise InternalError("sandbox-exec report is not a JSON object")

    status = classify(report, expected_output, mode)
    memory_source = report.get("memory_source")
    if memory_source not in MEMORY_SOURCES:
        raise InternalError(f"unknown memory_source: {memory_source!r}")

    error_message = None
    if status in (Verdict.RUNTIME_ERROR, Verdict.SYNTAX_ERROR):
        error_message = _last_stderr_line(report) or None
    return TestCaseResult(
        status=status,
        actual_output=str(report.get("stdout") or ""),
        is_truncated=bool(report.get("output_truncated")),
        exec_time_ms=_non_negative_int(report, "wall_ms"),
        memory_kb=_non_negative_int(report, "memory_kb"),
        memory_source=memory_source,
        error_message=error_message,
        syscall_nr=_syscall_nr(report) if status is Verdict.SECURITY_VIOLATION else None,
    )
