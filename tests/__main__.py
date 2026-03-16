"""
Entry point for running the test suite directly:

    python -m tests                         # run all tests
    python -m tests -v                      # verbose
    python -m tests -k test_health          # filter by name
    python -m tests -m "not ws"             # skip WebSocket tests
    python -m tests --tb=short              # short tracebacks
    python -m tests test_payments_create.py # single file

All extra arguments are forwarded to pytest as-is.
"""

import sys
from pathlib import Path

import pytest

# Ensure the tests directory is on sys.path so `import config` works
# regardless of where the user runs the command from.
tests_dir = Path(__file__).parent
if str(tests_dir) not in sys.path:
    sys.path.insert(0, str(tests_dir))


def main():
    args = sys.argv[1:]

    # Default to running all tests in this directory with a clean output style.
    if not args:
        args = [str(tests_dir), "-v", "--tb=short"]
    else:
        # If the user passes file names without paths, resolve them relative to tests/.
        resolved = []
        for arg in args:
            p = tests_dir / arg
            if p.exists() and not arg.startswith("-"):
                resolved.append(str(p))
            else:
                resolved.append(arg)
        args = resolved

    sys.exit(pytest.main(args))


if __name__ == "__main__":
    main()
