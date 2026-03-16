"""
Tests for GET /v1/payments, GET /v1/payments/{id}, GET /v1/payments/{id}/events

Covers retrieval, listing, pagination, and filtering.
"""

import requests
import pytest
import config


def _create_payment(**overrides) -> dict:
    payload = {
        "dest_asset_id": config.DEST_ASSET_ID,
        "dest_chain": config.DEST_CHAIN,
        "dest_address": config.DEST_ADDRESS,
        **overrides,
    }
    resp = requests.post(
        f"{config.V1}/payments",
        json=payload,
        headers=config.AUTH_HEADERS,
        timeout=config.REQUEST_TIMEOUT,
    )
    resp.raise_for_status()
    return resp.json()


@pytest.fixture(scope="module")
def payment():
    return _create_payment(metadata={"test_read": True})


class TestGetPayment:
    def test_get_existing_payment_returns_200(self, payment):
        resp = requests.get(
            f"{config.V1}/payments/{payment['id']}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200

    def test_get_returns_correct_id(self, payment):
        resp = requests.get(
            f"{config.V1}/payments/{payment['id']}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.json()["id"] == payment["id"]

    def test_get_nonexistent_returns_404(self):
        fake_id = "00000000-0000-0000-0000-000000000000"
        resp = requests.get(
            f"{config.V1}/payments/{fake_id}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 404

    def test_get_invalid_uuid_returns_400(self):
        resp = requests.get(
            f"{config.V1}/payments/not-a-uuid",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 400

    def test_get_unauthenticated_returns_401(self, payment):
        resp = requests.get(
            f"{config.V1}/payments/{payment['id']}",
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 401


class TestGetPaymentPublic:
    """The /v1/pay/{id} endpoint is public (no auth) and returns a subset of fields."""

    def test_public_endpoint_returns_200(self, payment):
        resp = requests.get(
            f"{config.BASE_URL}/v1/pay/{payment['id']}",
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200

    def test_public_endpoint_omits_callback_url(self, payment):
        # Create a payment with a callback URL.
        p = _create_payment(callback_url="https://example.com/cb")
        resp = requests.get(
            f"{config.BASE_URL}/v1/pay/{p['id']}",
            timeout=config.REQUEST_TIMEOUT,
        )
        body = resp.json()
        assert not body.get("callback_url"), "callback_url should be stripped from public response"

    def test_public_endpoint_omits_metadata(self, payment):
        p = _create_payment(metadata={"secret": "data"})
        resp = requests.get(
            f"{config.BASE_URL}/v1/pay/{p['id']}",
            timeout=config.REQUEST_TIMEOUT,
        )
        body = resp.json()
        assert not body.get("metadata"), "metadata should be stripped from public response"

    def test_public_nonexistent_returns_404(self):
        resp = requests.get(
            f"{config.BASE_URL}/v1/pay/00000000-0000-0000-0000-000000000000",
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 404


class TestListPayments:
    def test_list_returns_200(self):
        resp = requests.get(
            f"{config.V1}/payments",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200

    def test_list_has_payments_array(self):
        resp = requests.get(
            f"{config.V1}/payments",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        body = resp.json()
        assert "payments" in body
        assert isinstance(body["payments"], list)

    def test_list_has_pagination_fields(self):
        resp = requests.get(
            f"{config.V1}/payments",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        body = resp.json()
        assert "total" in body
        assert "page" in body
        assert "page_size" in body

    def test_list_default_page_is_1(self):
        resp = requests.get(
            f"{config.V1}/payments",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.json()["page"] == 1

    def test_list_page_size_param(self):
        resp = requests.get(
            f"{config.V1}/payments",
            params={"page_size": 2},
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        body = resp.json()
        assert len(body["payments"]) <= 2

    def test_list_status_filter(self):
        resp = requests.get(
            f"{config.V1}/payments",
            params={"status": "AWAITING_DEPOSIT"},
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        body = resp.json()
        for p in body["payments"]:
            assert p["status"] == "AWAITING_DEPOSIT"

    def test_list_created_payment_appears(self, payment):
        # Fetch enough rows to find our payment.
        resp = requests.get(
            f"{config.V1}/payments",
            params={"page_size": 100},
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        ids = {p["id"] for p in resp.json()["payments"]}
        assert payment["id"] in ids


class TestPaymentEvents:
    def test_events_endpoint_returns_200(self, payment):
        resp = requests.get(
            f"{config.V1}/payments/{payment['id']}/events",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200

    def test_events_has_events_array(self, payment):
        resp = requests.get(
            f"{config.V1}/payments/{payment['id']}/events",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        body = resp.json()
        assert "events" in body
        assert isinstance(body["events"], list)

    def test_creation_event_exists(self, payment):
        resp = requests.get(
            f"{config.V1}/payments/{payment['id']}/events",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        events = resp.json()["events"]
        assert len(events) >= 1, "At least one event should exist after creation"
