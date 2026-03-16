"""
pytest conftest - shared fixtures and session-level setup.
"""

import pytest
import requests
import config as cfg


def pytest_configure(config):
    """Register custom markers."""
    config.addinivalue_line("markers", "slow: marks tests that take >2s (deselect with -m 'not slow')")
    config.addinivalue_line("markers", "ws: marks WebSocket tests")


@pytest.fixture(scope="session", autouse=True)
def check_server_reachable():
    """Abort the session early if the server is not reachable."""
    try:
        resp = requests.get(f"{cfg.V1}/health", timeout=5)
        resp.raise_for_status()
    except Exception as exc:
        pytest.exit(
            f"\n\nServer not reachable at {cfg.BASE_URL}\n"
            f"Start the server first, then re-run the tests.\n"
            f"Error: {exc}\n",
            returncode=1,
        )
