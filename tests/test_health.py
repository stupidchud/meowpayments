"""
Tests for GET /v1/health
"""

import requests
import pytest
import config


def test_health_returns_200():
    resp = requests.get(f"{config.V1}/health", timeout=config.REQUEST_TIMEOUT)
    assert resp.status_code == 200


def test_health_body_has_status_ok():
    resp = requests.get(f"{config.V1}/health", timeout=config.REQUEST_TIMEOUT)
    body = resp.json()
    assert body.get("status") == "ok"


def test_health_body_has_timestamp():
    resp = requests.get(f"{config.V1}/health", timeout=config.REQUEST_TIMEOUT)
    body = resp.json()
    assert "timestamp" in body
    assert body["timestamp"]  # non-empty
