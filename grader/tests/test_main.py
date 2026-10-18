import json
import subprocess
import sys

import harness.__main__ as cli
from harness.verdict import InternalError

ENV = {"SANDBOX_EXEC": "/opt/sandbox-exec", "SANDBOX_CGROUP_ROOT": "/sys/fs/cgroup/grader"}
REPORT = {"submission_id": 7, "total_score": 3, "max_score": 5, "results": []}


def write_files(tmp_path):
    definition = tmp_path / "definition.json"
    definition.write_text(
        json.dumps(
            {
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
                "test_cases": [],
            }
        ),
        encoding="utf-8",
    )
    code = tmp_path / "solution.py"
    code.write_text("print(1)\n", encoding="utf-8")
    return str(definition), str(code)


def use_env(monkeypatch):
    for key, value in ENV.items():
        monkeypatch.setenv(key, value)


def test_success_writes_one_json_document_and_returns_zero(monkeypatch, tmp_path, capsys):
    use_env(monkeypatch)
    monkeypatch.setattr(cli, "grade", lambda definition, code, config: REPORT)
    definition, code = write_files(tmp_path)
    assert cli.main(["--definition", definition, "--code", code]) == 0
    out = capsys.readouterr().out
    assert json.loads(out) == REPORT
    assert out.count("\n") == 1


def test_internal_error_writes_nothing_to_stdout(monkeypatch, tmp_path, capsys):
    use_env(monkeypatch)

    def boom(definition, code, config):
        raise InternalError("sandbox-exec exited with status 70")

    monkeypatch.setattr(cli, "grade", boom)
    definition, code = write_files(tmp_path)
    assert cli.main(["--definition", definition, "--code", code]) == 1
    captured = capsys.readouterr()
    assert captured.out == ""
    assert "status 70" in captured.err


def test_missing_environment_returns_one(monkeypatch, tmp_path, capsys):
    monkeypatch.delenv("SANDBOX_EXEC", raising=False)
    monkeypatch.delenv("SANDBOX_CGROUP_ROOT", raising=False)
    definition, code = write_files(tmp_path)
    assert cli.main(["--definition", definition, "--code", code]) == 1
    assert capsys.readouterr().out == ""


def test_missing_code_file_returns_one(monkeypatch, tmp_path, capsys):
    use_env(monkeypatch)
    definition, _ = write_files(tmp_path)
    assert cli.main(["--definition", definition, "--code", str(tmp_path / "none.py")]) == 1
    assert capsys.readouterr().out == ""


def test_missing_definition_file_returns_one(monkeypatch, tmp_path, capsys):
    use_env(monkeypatch)
    _, code = write_files(tmp_path)
    assert cli.main(["--definition", str(tmp_path / "none.json"), "--code", code]) == 1
    assert capsys.readouterr().out == ""


def test_python_dash_m_harness_fails_without_arguments(tmp_path):
    proc = subprocess.run(
        [sys.executable, "-m", "harness"],
        capture_output=True,
        cwd=tmp_path,
        check=False,
    )
    assert proc.returncode != 0
    assert proc.stdout == b""
