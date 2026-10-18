"""Grade one submission: run every test case and build the report (SRS Appendix C)."""

import json
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from harness.compare import MatchMode
from harness.runner import Limits, SandboxConfig, TestCaseResult, run_test_case
from harness.verdict import InternalError, Verdict

DEFINITION_VERSION = 1


@dataclass(frozen=True)
class CaseDefinition:
    """One test case of the assignment definition."""

    id: int
    external_id: str
    input: str
    expected_output: str
    weight: int
    match_mode: MatchMode


@dataclass(frozen=True)
class Definition:
    """Everything needed to grade one submission, as written by the worker."""

    submission_id: int
    limits: Limits
    test_cases: tuple[CaseDefinition, ...]


def _require_int(data: Mapping[str, Any], key: str, minimum: int, where: str) -> int:
    value = data.get(key)
    if isinstance(value, bool) or not isinstance(value, int) or value < minimum:
        raise InternalError(f"{where}.{key} must be an integer >= {minimum}")
    return value


def _require_str(data: Mapping[str, Any], key: str, where: str) -> str:
    value = data.get(key)
    if not isinstance(value, str):
        raise InternalError(f"{where}.{key} must be a string")
    return value


def _parse_case(data: Any, index: int) -> CaseDefinition:
    where = f"test_cases[{index}]"
    if not isinstance(data, dict):
        raise InternalError(f"{where} must be an object")
    try:
        mode = MatchMode(data.get("match_mode"))
    except ValueError as exc:
        raise InternalError(f"{where}.match_mode is not a known mode") from exc
    return CaseDefinition(
        id=_require_int(data, "id", 0, where),
        external_id=_require_str(data, "external_id", where),
        input=_require_str(data, "input", where),
        expected_output=_require_str(data, "expected_output", where),
        weight=_require_int(data, "weight", 0, where),
        match_mode=mode,
    )


def parse_definition(data: Any) -> Definition:
    """Validate the decoded definition file and return it. Raises InternalError if invalid."""
    if not isinstance(data, dict):
        raise InternalError("definition must be a JSON object")
    if data.get("schema_version") != DEFINITION_VERSION:
        raise InternalError(f"definition schema_version must be {DEFINITION_VERSION}")
    submission_id = _require_int(data, "submission_id", 0, "definition")
    limits = data.get("resource_limit")
    if not isinstance(limits, dict):
        raise InternalError("definition.resource_limit must be an object")
    raw_cases = data.get("test_cases")
    if not isinstance(raw_cases, list):
        raise InternalError("definition.test_cases must be a list")

    cases = tuple(_parse_case(raw, i) for i, raw in enumerate(raw_cases))
    if len({case.id for case in cases}) != len(cases):
        raise InternalError("definition lists the same test case id twice")
    return Definition(submission_id, Limits.from_dict(limits), cases)


def load_definition(path: str) -> Definition:
    """Read and validate the definition file the worker passed with --definition."""
    try:
        text = Path(path).read_text(encoding="utf-8")
        data = json.loads(text)
    except (OSError, ValueError) as exc:
        raise InternalError(f"cannot read the definition file: {exc}") from exc
    return parse_definition(data)


def _result_dict(case: CaseDefinition, result: TestCaseResult) -> dict[str, Any]:
    # A test case is all or nothing: the full weight when it passes, otherwise zero.
    score = case.weight if result.status is Verdict.PASSED else 0
    item: dict[str, Any] = {
        "testcase_id": case.id,
        "external_id": case.external_id,
        "status": result.status.value,
        "score": score,
        "actual_output": result.actual_output,
        "is_truncated": result.is_truncated,
        "exec_time_ms": result.exec_time_ms,
        "memory_kb": result.memory_kb,
        "memory_source": result.memory_source,
    }
    if result.error_message is not None:
        item["error_message"] = result.error_message
    if result.status is Verdict.SECURITY_VIOLATION and result.syscall_nr is not None:
        item["violation"] = {"syscall_nr": result.syscall_nr}
    return item


def grade(definition: Definition, code_path: str, config: SandboxConfig) -> dict[str, Any]:
    """Run every test case one after another and return the report.

    Test cases run sequentially so resource measurements do not interfere. An InternalError
    from any test case stops the run: the submission is then not graded at all.
    """
    results = []
    for case in definition.test_cases:
        outcome = run_test_case(
            config,
            definition.limits,
            code_path,
            case.input,
            case.expected_output,
            case.match_mode,
        )
        results.append(_result_dict(case, outcome))
    return {
        "submission_id": definition.submission_id,
        "total_score": sum(item["score"] for item in results),
        "max_score": sum(case.weight for case in definition.test_cases),
        "results": results,
    }
