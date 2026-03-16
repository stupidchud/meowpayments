"""
Tests for POST /v1/payments

Covers the payment creation flow: validation, successful creation,
and the structure of the returned payment object.
"""

import requests
import pytest
import config


def _create_payment(**overrides) -> requests.Response:
    """Helper: POST /v1/payments with sensible defaults, merged with overrides."""
    payload = {
        "dest_asset_id": config.DEST_ASSET_ID,
        "dest_chain": config.DEST_CHAIN,
        "dest_address": config.DEST_ADDRESS,
        "amount_usd": "10.00",
        "expires_in_seconds": 3600,
        "metadata": {"test": True},
        **overrides,
    }
    return requests.post(
        f"{config.V1}/payments",
        json=payload,
        headers=config.AUTH_HEADERS,
        timeout=config.REQUEST_TIMEOUT,
    )


@pytest.fixture(scope="module")
def created_payment():
    """A single successfully created payment, shared across tests in this module."""
    resp = _create_payment()
    assert resp.status_code == 200, f"Setup failed: {resp.text}"
    return resp.json()


class TestPaymentCreationValidation:
    def test_missing_dest_asset_returns_400(self):
        resp = _create_payment(dest_asset_id=None)
        # Remove key entirely
        payload = {
            "dest_chain": config.DEST_CHAIN,
            "dest_address": config.DEST_ADDRESS,
        }
        resp = requests.post(
            f"{config.V1}/payments",
            json=payload,
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code in (400, 422)

    def test_missing_dest_chain_returns_400(self):
        payload = {
            "dest_asset_id": config.DEST_ASSET_ID,
            "dest_address": config.DEST_ADDRESS,
        }
        resp = requests.post(
            f"{config.V1}/payments",
            json=payload,
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code in (400, 422)

    def test_missing_dest_address_returns_400(self):
        payload = {
            "dest_asset_id": config.DEST_ASSET_ID,
            "dest_chain": config.DEST_CHAIN,
        }
        resp = requests.post(
            f"{config.V1}/payments",
            json=payload,
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code in (400, 422)

    def test_unauthenticated_returns_401(self):
        payload = {
            "dest_asset_id": config.DEST_ASSET_ID,
            "dest_chain": config.DEST_CHAIN,
            "dest_address": config.DEST_ADDRESS,
        }
        resp = requests.post(
            f"{config.V1}/payments",
            json=payload,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 401


class TestPaymentCreationResponse:
    def test_returns_200(self):
        resp = _create_payment()
        assert resp.status_code == 200

    def test_response_has_id(self, created_payment):
        assert "id" in created_payment
        assert created_payment["id"]

    def test_response_id_is_uuid(self, created_payment):
        import uuid
        uuid.UUID(created_payment["id"])  # raises ValueError if not valid UUID

    def test_response_status_is_awaiting_deposit(self, created_payment):
        assert created_payment["status"] == "AWAITING_DEPOSIT"

    def test_response_has_deposit_address(self, created_payment):
        assert "deposit_address" in created_payment
        assert created_payment["deposit_address"], "deposit_address should be non-empty"

    def test_response_has_payment_url(self, created_payment):
        assert "payment_url" in created_payment
        assert created_payment["payment_url"].startswith(config.BASE_URL)

    def test_response_has_expires_at(self, created_payment):
        assert "expires_at" in created_payment
        assert created_payment["expires_at"]

    def test_response_dest_fields_match_request(self, created_payment):
        assert created_payment["dest_asset_id"] == config.DEST_ASSET_ID
        assert created_payment["dest_chain"] == config.DEST_CHAIN
        assert created_payment["dest_address"] == config.DEST_ADDRESS

    def test_response_amount_usd_echoed(self, created_payment):
        assert "amount_usd" in created_payment
        assert created_payment["amount_usd"] is not None

    def test_payment_url_contains_id(self, created_payment):
        assert created_payment["id"] in created_payment["payment_url"]

    def test_origin_asset_id_optional(self, created_payment):
        """Omitting origin_asset_id should succeed (any-input mode); field absent or empty."""
        assert created_payment.get("origin_asset_id", "") == ""

    @pytest.mark.skipif(not config.ORIGIN_ASSET_ID, reason="ORIGIN_ASSET_ID not configured")
    def test_origin_asset_id_returns_native_chain_deposit_address(self):
        """With origin_asset_id + origin_refund_address the API should return
        a native chain deposit address (not the NEAR Intents virtual hex hash)."""
        resp = _create_payment(
            origin_asset_id=config.ORIGIN_ASSET_ID,
            origin_refund_address=config.ORIGIN_REFUND_ADDRESS,
        )
        assert resp.status_code == 200, resp.text
        data = resp.json()
        assert data.get("origin_asset_id") == config.ORIGIN_ASSET_ID
        # Native chain addresses are not 64-char hex hashes (which is the
        # NEAR Intents virtual address format used in any-input mode).
        deposit = data.get("deposit_address", "")
        assert deposit, "deposit_address should be non-empty"
        assert len(deposit) != 64 or not all(c in "0123456789abcdef" for c in deposit), (
            "deposit_address looks like a NEAR Intents virtual address - "
            "expected a native chain address (0x..., base58, etc.)"
        )

    @pytest.mark.skipif(not config.ORIGIN_ASSET_ID, reason="ORIGIN_ASSET_ID not configured")
    def test_origin_asset_id_without_refund_address_returns_error(self):
        """NEAR Intents EXACT_INPUT+ORIGIN_CHAIN requires a valid refundTo address.
        Omitting origin_refund_address must return an error (502 from upstream)."""
        resp = _create_payment(origin_asset_id=config.ORIGIN_ASSET_ID)
        assert resp.status_code in (400, 502), (
            f"Expected error when refund address is missing, got {resp.status_code}: {resp.text}"
        )

    def test_expires_in_respected(self):
        """Payments with custom expires_in_seconds should have a later expiry."""
        from datetime import datetime, timezone

        short_resp = _create_payment(expires_in_seconds=600).json()
        long_resp = _create_payment(expires_in_seconds=7200).json()

        import re
        def _parse_dt(s):
            # Truncate sub-second fraction to 6 digits (microseconds) for Python 3.10 compat.
            s = re.sub(r'(\.\d{6})\d+', r'\1', s).replace("Z", "+00:00")
            return datetime.fromisoformat(s)

        short_exp = _parse_dt(short_resp["expires_at"])
        long_exp = _parse_dt(long_resp["expires_at"])

        assert long_exp > short_exp
