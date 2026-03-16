"""
Tests for GET /v1/tokens

This endpoint is public (no auth required) and returns the cached
list of supported assets from the NEAR Intents 1-click API.
"""

import requests
import pytest
import config


@pytest.fixture(scope="module")
def tokens():
    resp = requests.get(f"{config.V1}/tokens", timeout=config.REQUEST_TIMEOUT)
    resp.raise_for_status()
    return resp.json()


class TestTokensEndpoint:
    def test_returns_200_without_auth(self):
        resp = requests.get(f"{config.V1}/tokens", timeout=config.REQUEST_TIMEOUT)
        assert resp.status_code == 200

    def test_response_has_tokens_list(self, tokens):
        assert "tokens" in tokens
        assert isinstance(tokens["tokens"], list)

    def test_response_has_count(self, tokens):
        assert "count" in tokens
        assert tokens["count"] == len(tokens["tokens"])

    def test_tokens_list_is_non_empty(self, tokens):
        assert len(tokens["tokens"]) > 0, "Expected at least one supported token"

    def test_token_has_required_fields(self, tokens):
        required = {"asset_id", "symbol", "blockchain", "decimals", "price_usd"}
        for token in tokens["tokens"]:
            missing = required - token.keys()
            assert not missing, f"Token missing fields {missing}: {token}"

    def test_token_decimals_are_positive(self, tokens):
        for token in tokens["tokens"]:
            assert token["decimals"] >= 0, f"Negative decimals on {token['asset_id']}"

    def test_configured_dest_asset_exists(self, tokens):
        """The DEST_ASSET_ID from config must be in the token list."""
        ids = {t["asset_id"] for t in tokens["tokens"]}
        assert config.DEST_ASSET_ID in ids, (
            f"DEST_ASSET_ID '{config.DEST_ASSET_ID}' not found in token list. "
            "Update your tests/.env."
        )
