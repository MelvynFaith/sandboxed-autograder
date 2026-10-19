"""Progress lines for the dashboard (FR-08)."""

import json
import os
from collections.abc import Callable, Mapping

ENV_PROGRESS_FD = "HARNESS_PROGRESS_FD"
# Descriptors 0, 1 and 2 are stdin, stdout (the report) and stderr: never write progress there.
FIRST_FREE_FD = 3


def progress_writer(env: Mapping[str, str]) -> Callable[[int], None] | None:
    """Return a function that reports "test case n is about to run", or None.

    HARNESS_PROGRESS_FD names an open pipe. The returned function writes one line
    {"current_test": n} to it, with n counting from 1. Progress is for display only, so
    nothing here can fail the grading: a missing or invalid variable gives None, and after
    the first failed write (closed or broken pipe) the function does nothing any more.
    The descriptor is never passed on to sandbox-exec (subprocess closes it by default).
    """
    raw = env.get(ENV_PROGRESS_FD)
    if raw is None:
        return None
    try:
        fd = int(raw)
    except ValueError:
        return None
    if fd < FIRST_FREE_FD:
        return None

    broken = False

    def write(number: int) -> None:
        nonlocal broken
        if broken:
            return
        line = json.dumps({"current_test": number}) + "\n"
        try:
            os.write(fd, line.encode("ascii"))
        except OSError:
            broken = True

    return write
