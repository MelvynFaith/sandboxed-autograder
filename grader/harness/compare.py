"""Compare actual program output with the expected output."""

from enum import Enum


class MatchMode(str, Enum):
    """Per-test-case comparison mode. Values match the YAML and SQL enums."""

    EXACT = "EXACT"
    WHITESPACE_INSENSITIVE = "WHITESPACE_INSENSITIVE"


def outputs_match(actual: str, expected: str, mode: MatchMode) -> bool:
    """Return True if actual output matches expected under the given mode.

    EXACT: the two strings are identical.
    WHITESPACE_INSENSITIVE: equal after splitting on any run of whitespace, so differences
    in spaces, tabs, newlines, and leading or trailing whitespace are ignored. Tokens are
    never merged: "12" does not match "1 2".
    """
    mode = MatchMode(mode)
    if mode is MatchMode.EXACT:
        return actual == expected
    return actual.split() == expected.split()
