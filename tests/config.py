"""
Test suite configuration.

Values are loaded from tests/.env (or environment variables).
Run `cp tests/.env.example tests/.env` and fill in your values before testing.
"""

import os
from pathlib import Path

from dotenv import load_dotenv

# Load tests/.env relative to this file, falling back to process environment.
_env_path = Path(__file__).parent / ".env"
load_dotenv(_env_path, override=False)


def _require(key: str) -> str:
    val = os.getenv(key)
    if not val:
        raise EnvironmentError(
            f"Required env var '{key}' is not set. "
            f"Copy tests/.env.example → tests/.env and fill it in."
        )
    return val


def _optional(key: str, default: str = "") -> str:
    return os.getenv(key, default)


#
# Server
#
BASE_URL: str = _optional("BASE_URL", "http://localhost:8080").rstrip("/")
API_KEY: str = _require("API_KEY")

# Derived
V1 = f"{BASE_URL}/v1"
AUTH_HEADERS = {
    "X-API-Key": API_KEY,
    "Content-Type": "application/json",
}

#
# Fixtures
#
DEST_ASSET_ID: str = _require("DEST_ASSET_ID")
DEST_CHAIN: str = _require("DEST_CHAIN")
DEST_ADDRESS: str = _require("DEST_ADDRESS")

# Optional: a specific origin asset to test chain-specific deposit addresses.
# Must be a valid defuse asset ID from GET /v1/tokens, e.g. USDT on Ethereum.
# If unset, origin_asset_id tests are skipped.
ORIGIN_ASSET_ID: str = _optional("ORIGIN_ASSET_ID")
# A valid refund address on the origin chain (required when ORIGIN_ASSET_ID is set).
ORIGIN_REFUND_ADDRESS: str = _optional("ORIGIN_REFUND_ADDRESS")

#
# Timing
#
REQUEST_TIMEOUT: int = int(_optional("REQUEST_TIMEOUT_SECONDS", "10"))
WS_TIMEOUT: int = int(_optional("WS_TIMEOUT_SECONDS", "5"))
