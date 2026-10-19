import os

import pytest

from harness.progress import progress_writer


def test_no_variable_means_no_progress():
    assert progress_writer({}) is None


@pytest.mark.parametrize("value", ["", "abc", "-1", "0", "1", "2"])
def test_invalid_descriptor_means_no_progress(value):
    # 0, 1 and 2 are never used: standard output carries the report.
    assert progress_writer({"HARNESS_PROGRESS_FD": value}) is None


def test_lines_are_written_to_the_descriptor():
    read_end, write_end = os.pipe()
    try:
        write = progress_writer({"HARNESS_PROGRESS_FD": str(write_end)})
        assert write is not None
        write(1)
        write(2)
        os.close(write_end)
        write_end = -1
        assert os.read(read_end, 1024) == b'{"current_test": 1}\n{"current_test": 2}\n'
    finally:
        os.close(read_end)
        if write_end >= 0:
            os.close(write_end)


def test_broken_pipe_does_not_raise():
    read_end, write_end = os.pipe()
    write = progress_writer({"HARNESS_PROGRESS_FD": str(write_end)})
    os.close(read_end)
    try:
        write(1)
        write(2)
    finally:
        os.close(write_end)


def test_closed_descriptor_does_not_raise():
    read_end, write_end = os.pipe()
    os.close(read_end)
    os.close(write_end)
    write = progress_writer({"HARNESS_PROGRESS_FD": str(write_end)})
    write(1)
