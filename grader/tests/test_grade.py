import json

import pytest

from harness import grade as grade_module
from harness.compare import MatchMode
from harness.grade import grade, load_definition, parse_definition
from harness.runner import SandboxConfig, TestCaseResult
from harness.verdict import InternalError, Verdict

CONFIG = SandboxConfig(sandbox_exec="/opt/sandbox-exec", cgroup_root="/sys/fs/cgroup/grader")
STATUSES = {v.value for v in Verdict}
MEMORY_SOURCES = {"CGROUP_PEAK", "CGROUP_SAMPLED", "RUSAGE"}


def make_data(**overrides):
    data = {
        "schema_version": 1,
        "submission_id": 7,
        "resource_limit": {
            "cpu_time_ms": 2000,
            "memory_mb": 64,
            "max_processes": 5,
            "timeout_s": 5,
            "max_output_bytes": 1048576,
            "tmpfs_size_mb": 32,
        },
        "test_cases": [
            {
                "id": 11,
                "external_id": "tc_01",
                "input": "5\n",
                "expected_output": "120\n",
                "weight": 3,
                "match_mode": "EXACT",
            },
            {
                "id": 12,
                "external_id": "tc_02",
                "input": "0\n",
                "expected_output": "1\n",
                "weight": 2,
                "match_mode": "WHITESPACE_INSENSITIVE",
            },
        ],
    }
    data.update(overrides)
    return data


def make_result(status=Verdict.PASSED, **overrides):
    fields = {
        "status": status,
        "actual_output": "120\n",
        "is_truncated": False,
        "exec_time_ms": 100,
        "memory_kb": 9000,
        "memory_source": "CGROUP_PEAK",
    }
    fields.update(overrides)
    return TestCaseResult(**fields)


def install_runner(monkeypatch, results_by_input):
    calls = []

    def fake(config, limits, code_path, input_text, expected_output, mode):
        calls.append((config, limits, code_path, input_text, expected_output, mode))
        return results_by_input[input_text]

    monkeypatch.setattr(grade_module, "run_test_case", fake)
    return calls


def check_like_the_scheduler(data, report):
    """The same rules as Report.Check in scheduler/internal/harness/report.go."""
    assert report["submission_id"] == data["submission_id"]
    weights = {tc["id"]: tc["weight"] for tc in data["test_cases"]}
    assert len(report["results"]) == len(weights)
    seen = set()
    total = 0
    for res in report["results"]:
        assert res["testcase_id"] in weights
        assert res["testcase_id"] not in seen
        seen.add(res["testcase_id"])
        assert res["status"] in STATUSES
        assert res["memory_source"] in MEMORY_SOURCES
        assert 0 <= res["score"] <= weights[res["testcase_id"]]
        assert res["exec_time_ms"] >= 0
        assert res["memory_kb"] >= 0
        total += res["score"]
    assert report["max_score"] == sum(weights.values())
    assert report["total_score"] == total


def test_parse_definition_ok():
    definition = parse_definition(make_data())
    assert definition.submission_id == 7
    assert definition.limits.memory_mb == 64
    assert [c.external_id for c in definition.test_cases] == ["tc_01", "tc_02"]
    assert definition.test_cases[1].match_mode is MatchMode.WHITESPACE_INSENSITIVE


def test_parse_definition_accepts_no_test_cases():
    assert parse_definition(make_data(test_cases=[])).test_cases == ()


@pytest.mark.parametrize(
    "bad",
    [
        [],
        make_data(schema_version=2),
        make_data(submission_id="7"),
        make_data(submission_id=True),
        make_data(resource_limit=None),
        make_data(test_cases="nope"),
    ],
)
def test_parse_definition_rejects_bad_top_level(bad):
    with pytest.raises(InternalError):
        parse_definition(bad)


@pytest.mark.parametrize(
    "change",
    [
        {"match_mode": "FUZZY"},
        {"weight": -1},
        {"weight": 1.5},
        {"id": "x"},
        {"input": None},
        {"expected_output": 3},
        {"external_id": None},
    ],
)
def test_parse_definition_rejects_bad_test_case(change):
    data = make_data()
    data["test_cases"][0].update(change)
    with pytest.raises(InternalError):
        parse_definition(data)


def test_parse_definition_rejects_non_object_test_case():
    with pytest.raises(InternalError):
        parse_definition(make_data(test_cases=["x"]))


def test_parse_definition_rejects_duplicate_ids():
    data = make_data()
    data["test_cases"][1]["id"] = 11
    with pytest.raises(InternalError):
        parse_definition(data)


def test_parse_definition_rejects_bad_limits():
    data = make_data()
    data["resource_limit"]["memory_mb"] = 0
    with pytest.raises(InternalError):
        parse_definition(data)


def test_load_definition_reads_a_file(tmp_path):
    path = tmp_path / "definition.json"
    path.write_text(json.dumps(make_data()), encoding="utf-8")
    assert load_definition(str(path)).submission_id == 7


def test_load_definition_missing_file_raises(tmp_path):
    with pytest.raises(InternalError):
        load_definition(str(tmp_path / "missing.json"))


def test_load_definition_invalid_json_raises(tmp_path):
    path = tmp_path / "definition.json"
    path.write_text("{not json", encoding="utf-8")
    with pytest.raises(InternalError):
        load_definition(str(path))


def test_scores_are_all_or_nothing_and_totals_add_up(monkeypatch):
    install_runner(
        monkeypatch,
        {"5\n": make_result(Verdict.PASSED), "0\n": make_result(Verdict.FAILED)},
    )
    data = make_data()
    report = grade(parse_definition(data), "/tmp/solution.py", CONFIG)
    assert [r["score"] for r in report["results"]] == [3, 0]
    assert report["total_score"] == 3
    assert report["max_score"] == 5
    check_like_the_scheduler(data, report)


def test_all_passed_gets_the_full_score(monkeypatch):
    install_runner(monkeypatch, {"5\n": make_result(), "0\n": make_result()})
    data = make_data()
    report = grade(parse_definition(data), "/tmp/solution.py", CONFIG)
    assert report["total_score"] == report["max_score"] == 5
    check_like_the_scheduler(data, report)


def test_test_cases_run_in_order_with_their_own_input(monkeypatch):
    calls = install_runner(monkeypatch, {"5\n": make_result(), "0\n": make_result()})
    grade(parse_definition(make_data()), "/tmp/solution.py", CONFIG)
    assert [c[3] for c in calls] == ["5\n", "0\n"]
    assert [c[4] for c in calls] == ["120\n", "1\n"]
    assert [c[5] for c in calls] == [MatchMode.EXACT, MatchMode.WHITESPACE_INSENSITIVE]
    assert all(c[2] == "/tmp/solution.py" and c[0] == CONFIG for c in calls)


def test_result_fields_are_copied_from_the_runner(monkeypatch):
    install_runner(
        monkeypatch,
        {
            "5\n": make_result(
                actual_output="x", is_truncated=True, exec_time_ms=42, memory_kb=7,
                memory_source="RUSAGE",
            ),
            "0\n": make_result(),
        },
    )  # fmt: skip
    first = grade(parse_definition(make_data()), "/tmp/s.py", CONFIG)["results"][0]
    assert first["testcase_id"] == 11
    assert first["external_id"] == "tc_01"
    assert first["actual_output"] == "x"
    assert first["is_truncated"] is True
    assert first["exec_time_ms"] == 42
    assert first["memory_kb"] == 7
    assert first["memory_source"] == "RUSAGE"


def test_error_message_is_only_present_when_there_is_one(monkeypatch):
    install_runner(
        monkeypatch,
        {
            "5\n": make_result(Verdict.RUNTIME_ERROR, error_message="ValueError: bad"),
            "0\n": make_result(),
        },
    )
    results = grade(parse_definition(make_data()), "/tmp/s.py", CONFIG)["results"]
    assert results[0]["error_message"] == "ValueError: bad"
    assert "error_message" not in results[1]


def test_violation_is_only_present_for_security_violation(monkeypatch):
    install_runner(
        monkeypatch,
        {
            "5\n": make_result(Verdict.SECURITY_VIOLATION, syscall_nr=101),
            "0\n": make_result(Verdict.FAILED, syscall_nr=5),
        },
    )
    results = grade(parse_definition(make_data()), "/tmp/s.py", CONFIG)["results"]
    assert results[0]["violation"] == {"syscall_nr": 101}
    assert "violation" not in results[1]


def test_report_has_no_expected_output(monkeypatch):
    install_runner(monkeypatch, {"5\n": make_result(), "0\n": make_result()})
    data = make_data()
    data["test_cases"][0]["expected_output"] = "HIDDEN-ANSWER-42"
    report = grade(parse_definition(data), "/tmp/s.py", CONFIG)
    assert "HIDDEN-ANSWER-42" not in json.dumps(report)


def test_report_is_json_serializable(monkeypatch):
    install_runner(monkeypatch, {"5\n": make_result(), "0\n": make_result()})
    report = grade(parse_definition(make_data()), "/tmp/s.py", CONFIG)
    assert json.loads(json.dumps(report)) == report


def test_internal_error_stops_the_run(monkeypatch):
    calls = []

    def fake(config, limits, code_path, input_text, expected_output, mode):
        calls.append(input_text)
        raise InternalError("sandbox-exec exited with status 70")

    monkeypatch.setattr(grade_module, "run_test_case", fake)
    with pytest.raises(InternalError):
        grade(parse_definition(make_data()), "/tmp/s.py", CONFIG)
    assert calls == ["5\n"]


def test_no_test_cases_gives_an_empty_report():
    report = grade(parse_definition(make_data(test_cases=[])), "/tmp/s.py", CONFIG)
    assert report == {"submission_id": 7, "total_score": 0, "max_score": 0, "results": []}
