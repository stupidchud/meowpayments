"""
Tests for DELETE /v1/payments/{id}

Covers cancellation, double-cancel, and auth requirements.
"""

import requests
import pytest
import config


def _create_payment() -> dict:
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


class TestCancelPayment:
    def test_cancel_returns_200(self):
        payment = _create_payment()
        resp = requests.delete(
            f"{config.V1}/payments/{payment['id']}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200

    def test_cancel_sets_status_failed(self):
        payment = _create_payment()
        resp = requests.delete(
            f"{config.V1}/payments/{payment['id']}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.json()["status"] == "FAILED"

    def test_cancel_nonexistent_returns_404(self):
        resp = requests.delete(
            f"{config.V1}/payments/00000000-0000-0000-0000-000000000000",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 404

    def test_cancel_invalid_uuid_returns_400(self):
        resp = requests.delete(
            f"{config.V1}/payments/not-a-uuid",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 400

    def test_cancel_unauthenticated_returns_401(self):
        payment = _create_payment()
        resp = requests.delete(
            f"{config.V1}/payments/{payment['id']}",
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 401

    def test_double_cancel_returns_409(self):
        """Cancelling an already-terminal payment should return 409 Conflict."""
        payment = _create_payment()
        # First cancel
        requests.delete(
            f"{config.V1}/payments/{payment['id']}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        # Second cancel
        resp = requests.delete(
            f"{config.V1}/payments/{payment['id']}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 409

    def test_cancelled_payment_still_readable(self):
        payment = _create_payment()
        requests.delete(
            f"{config.V1}/payments/{payment['id']}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        resp = requests.get(
            f"{config.V1}/payments/{payment['id']}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200
        assert resp.json()["status"] == "FAILED"
