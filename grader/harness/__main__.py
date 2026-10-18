"""Command line entry point: python -m harness --definition PATH --code PATH."""

import argparse
import json
import os
import sys
from collections.abc import Sequence
from pathlib import Path

from harness.grade import grade, load_definition
from harness.runner import SandboxConfig
from harness.verdict import InternalError


def main(argv: Sequence[str] | None = None) -> int:
    """Grade one submission. Write one JSON report to stdout and return 0, or return 1.

    On any failure nothing is written to stdout: the message goes to stderr and the exit
    status is not 0, so the worker marks the submission FAILED.
    """
    parser = argparse.ArgumentParser(prog="python -m harness", description=__doc__)
    parser.add_argument("--definition", required=True, help="JSON file with the assignment")
    parser.add_argument("--code", required=True, help="the student's source file")
    args = parser.parse_args(argv)

    try:
        config = SandboxConfig.from_env(os.environ)
        if not Path(args.code).is_file():
            raise InternalError("the code file does not exist")
        report = grade(load_definition(args.definition), args.code, config)
    except InternalError as exc:
        print(f"harness: {exc}", file=sys.stderr)
        return 1

    sys.stdout.write(json.dumps(report) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
