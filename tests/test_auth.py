"""
Tests for operator authentication (X-API-Key / Bearer token).

Verifies that protected routes reject missing or invalid keys
and accept the correct key.
"""

import requests
import pytest
import config


PROTECTED = f"{config.V1}/payments"


class TestMissingAuth:
    def test_no_key_returns_401(self):
        resp = requests.get(PROTECTED, timeout=config.REQUEST_TIMEOUT)
        assert resp.status_code == 401

    def test_wrong_key_returns_401(self):
        resp = requests.get(
            PROTECTED,
            headers={"X-API-Key": "definitely-wrong-key"},
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 401

    def test_empty_bearer_returns_401(self):
        resp = requests.get(
            PROTECTED,
            headers={"Authorization": "Bearer "},
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 401


class TestValidAuth:
    def test_xapikey_header_accepted(self):
        resp = requests.get(
            PROTECTED,
            headers={"X-API-Key": config.API_KEY},
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200

    def test_bearer_token_accepted(self):
        resp = requests.get(
            PROTECTED,
            headers={"Authorization": f"Bearer {config.API_KEY}"},
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200
