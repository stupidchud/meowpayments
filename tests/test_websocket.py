"""
Tests for WebSocket endpoints:
  - GET /v1/pay/{id}/ws   (public, per-payment)
  - GET /v1/ws            (operator, global)

Uses the websocket-client library for synchronous WS connections.
"""

import json
import threading
import time

import pytest
import requests
import websocket  # websocket-client

import config


def _ws_url(path: str) -> str:
    base = config.BASE_URL.replace("http://", "ws://").replace("https://", "wss://")
    return base + path


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


@pytest.fixture(scope="module")
def payment():
    return _create_payment()


class TestCustomerWebSocket:
    def test_connects_to_payment_ws(self, payment):
        """Customer WS for an existing payment should accept the connection."""
        url = _ws_url(f"/v1/pay/{payment['id']}/ws")
        received = []
        errors = []

        def on_message(ws, message):
            received.append(json.loads(message))
            ws.close()

        def on_error(ws, error):
            errors.append(error)

        ws = websocket.WebSocketApp(url, on_message=on_message, on_error=on_error)
        thread = threading.Thread(target=ws.run_forever)
        thread.daemon = True
        thread.start()
        thread.join(timeout=config.WS_TIMEOUT)
        ws.close()

        assert not errors, f"WebSocket errors: {errors}"

    def test_rejects_nonexistent_payment(self):
        """Connecting to a non-existent payment's WS should fail (HTTP 404 upgrade rejection)."""
        url = _ws_url("/v1/pay/00000000-0000-0000-0000-000000000000/ws")
        errors = []

        def on_error(ws, error):
            errors.append(error)

        ws = websocket.WebSocketApp(url, on_error=on_error)
        thread = threading.Thread(target=ws.run_forever)
        thread.daemon = True
        thread.start()
        thread.join(timeout=config.WS_TIMEOUT)
        ws.close()

        # We expect an error (connection rejected / 404)
        assert errors, "Expected an error for non-existent payment WS"


class TestOperatorWebSocket:
    def test_connects_with_valid_api_key_header(self):
        """Operator WS should accept connection when X-API-Key is passed as query param."""
        url = _ws_url(f"/v1/ws?token={config.API_KEY}")
        # Note: browser WS can't set headers, so the server accepts ?token= query param.
        errors = []

        ws = websocket.WebSocketApp(url, on_error=lambda ws, e: errors.append(e))
        thread = threading.Thread(target=ws.run_forever)
        thread.daemon = True
        thread.start()
        time.sleep(1)  # give it a moment to connect
        ws.close()
        thread.join(timeout=config.WS_TIMEOUT)

        assert not errors, f"Unexpected errors: {errors}"

    def test_rejects_missing_api_key(self):
        """Operator WS without auth should be rejected."""
        url = _ws_url("/v1/ws")
        errors = []

        ws = websocket.WebSocketApp(url, on_error=lambda ws, e: errors.append(e))
        thread = threading.Thread(target=ws.run_forever)
        thread.daemon = True
        thread.start()
        thread.join(timeout=config.WS_TIMEOUT)
        ws.close()

        assert errors, "Expected connection rejection without API key"

    def test_rejects_wrong_api_key(self):
        url = _ws_url("/v1/ws?token=totally-wrong-key")
        errors = []

        ws = websocket.WebSocketApp(url, on_error=lambda ws, e: errors.append(e))
        thread = threading.Thread(target=ws.run_forever)
        thread.daemon = True
        thread.start()
        thread.join(timeout=config.WS_TIMEOUT)
        ws.close()

        assert errors, "Expected connection rejection with wrong API key"
