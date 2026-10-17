import pytest

from harness.compare import MatchMode, outputs_match


def test_exact_identical():
    assert outputs_match("120\n", "120\n", MatchMode.EXACT)


def test_exact_rejects_missing_trailing_newline():
    assert not outputs_match("120", "120\n", MatchMode.EXACT)


def test_exact_rejects_extra_spaces():
    assert not outputs_match("1  2\n", "1 2\n", MatchMode.EXACT)


def test_whitespace_insensitive_ignores_trailing_newline():
    assert outputs_match("120", "120\n", MatchMode.WHITESPACE_INSENSITIVE)


def test_whitespace_insensitive_ignores_spacing():
    assert outputs_match("1   2\t3\n\n", "1 2 3\n", MatchMode.WHITESPACE_INSENSITIVE)


def test_whitespace_insensitive_still_detects_different_tokens():
    assert not outputs_match("1 2 4", "1 2 3", MatchMode.WHITESPACE_INSENSITIVE)


def test_whitespace_insensitive_does_not_merge_tokens():
    assert not outputs_match("12", "1 2", MatchMode.WHITESPACE_INSENSITIVE)


def test_empty_outputs_match():
    assert outputs_match("", "", MatchMode.EXACT)
    assert outputs_match("", "\n", MatchMode.WHITESPACE_INSENSITIVE)


def test_mode_accepts_plain_string():
    assert outputs_match("a", "a", "EXACT")


def test_invalid_mode_raises():
    with pytest.raises(ValueError):
        outputs_match("a", "a", "FUZZY")
