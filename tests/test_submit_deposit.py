"""
Tests for POST /v1/pay/{id}/submit

This is the public endpoint customers call after sending their transaction
to speed up detection. No auth required.
"""

import requests
import pytest
import config


@pytest.fixture(scope="module")
def payment():
    resp = requests.post(
        f"{config.V1}/payments",
        json={
            "dest_asset_id": config.DEST_ASSET_ID,
            "dest_chain": config.DEST_CHAIN,
            "dest_address": config.DEST_ADDRESS,
        },
        headers=config.AUTH_HEADERS,
        timeout=config.REQUEST_TIMEOUT,
    )
    resp.raise_for_status()
    return resp.json()


class TestSubmitDeposit:
    def test_submit_with_fake_hash_returns_200(self, payment):
        """
        Submitting a dummy tx hash should succeed at the API layer.
        The 1-click API may or may not reject it downstream, but our
        endpoint treats that as non-fatal and returns ok: true.
        """
        resp = requests.post(
            f"{config.BASE_URL}/v1/pay/{payment['id']}/submit",
            json={"tx_hash": "0x" + "aa" * 32},
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200

    def test_submit_returns_ok_true(self, payment):
        resp = requests.post(
            f"{config.BASE_URL}/v1/pay/{payment['id']}/submit",
            json={"tx_hash": "0x" + "bb" * 32},
            timeout=config.REQUEST_TIMEOUT,
        )
        body = resp.json()
        assert body.get("ok") is True

    def test_submit_missing_tx_hash_returns_400(self, payment):
        resp = requests.post(
            f"{config.BASE_URL}/v1/pay/{payment['id']}/submit",
            json={},
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code in (400, 422)

    def test_submit_nonexistent_payment_returns_404(self):
        resp = requests.post(
            f"{config.BASE_URL}/v1/pay/00000000-0000-0000-0000-000000000000/submit",
            json={"tx_hash": "0xdeadbeef"},
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 404

    def test_submit_no_auth_required(self, payment):
        """This endpoint must not require the API key."""
        resp = requests.post(
            f"{config.BASE_URL}/v1/pay/{payment['id']}/submit",
            json={"tx_hash": "0x" + "cc" * 32},
            # Deliberately no auth headers
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code != 401
