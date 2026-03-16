"""
Tests for origin_asset_id / chain-specific deposit address flow.

When origin_asset_id is supplied, meowpayments calls the NEAR Intents 1-click API
with EXACT_INPUT + ORIGIN_CHAIN, which returns a real native chain deposit address
(e.g. 0x... for Ethereum, base58 for Solana) instead of the generic NEAR Intents
virtual hex address used in ANY_INPUT mode.

All tests that actually hit the upstream 1-click API are marked with
@pytest.mark.skipif(not config.ORIGIN_ASSET_ID, ...) so they are skipped in
environments where the optional config is not set.

Known well-formed asset IDs (from GET /v1/tokens):
  ETH  USDT: nep141:eth-0xdac17f958d2ee523a2206206994597c13d831ec7.omft.near
  ETH  USDC: nep141:eth-0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48.omft.near
  SOL  USDC: nep141:sol-5ce3bf3a31af18be40ba30f721101b4341690186.omft.near (base58 program id)
  BASE USDC: nep141:base-0x833589fcd6edb6e08f4c7c32d4f71b54bda02913.omft.near
"""

import re
import requests
import pytest
import config


# ── helpers ──────────────────────────────────────────────────────────────────

def _create(**overrides) -> requests.Response:
    payload = {
        "dest_asset_id":  config.DEST_ASSET_ID,
        "dest_chain":     config.DEST_CHAIN,
        "dest_address":   config.DEST_ADDRESS,
        "amount_usd":     "20.00",
        "expires_in_seconds": 3600,
        **overrides,
    }
    return requests.post(
        f"{config.V1}/payments",
        json=payload,
        headers=config.AUTH_HEADERS,
        timeout=config.REQUEST_TIMEOUT,
    )


def _is_near_virtual_address(addr: str) -> bool:
    """Returns True if addr looks like a NEAR Intents virtual address (64 lowercase hex chars)."""
    return bool(re.fullmatch(r"[0-9a-f]{64}", addr))


def _is_evm_address(addr: str) -> bool:
    return bool(re.fullmatch(r"0x[0-9a-fA-F]{40}", addr))


def _is_solana_address(addr: str) -> bool:
    # Base58 characters, 32–44 chars
    base58_chars = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
    return 32 <= len(addr) <= 44 and all(c in base58_chars for c in addr)


# ── require-config skip marker ───────────────────────────────────────────────

needs_origin = pytest.mark.skipif(
    not config.ORIGIN_ASSET_ID,
    reason="ORIGIN_ASSET_ID not configured in tests/.env",
)


# ── baseline: any-input mode still works ─────────────────────────────────────

class TestAnyInputMode:
    """Without origin_asset_id the server must still work exactly as before."""

    def test_no_origin_returns_200(self):
        resp = _create()
        assert resp.status_code == 200, resp.text

    def test_no_origin_deposit_address_is_virtual(self):
        """ANY_INPUT mode returns the 64-char NEAR Intents virtual hex address."""
        data = _create().json()
        deposit = data.get("deposit_address", "")
        assert deposit, "deposit_address must be non-empty"
        assert _is_near_virtual_address(deposit), (
            f"Expected 64-char hex virtual address in ANY_INPUT mode, got: {deposit!r}"
        )

    def test_no_origin_origin_asset_id_absent(self):
        data = _create().json()
        assert data.get("origin_asset_id", "") == ""


# ── origin_asset_id field validation ─────────────────────────────────────────

class TestOriginAssetFieldValidation:
    def test_empty_string_origin_asset_treated_as_absent(self):
        """Explicitly passing an empty string should behave like omitting the field."""
        resp = _create(origin_asset_id="")
        assert resp.status_code == 200, resp.text
        data = resp.json()
        assert data.get("origin_asset_id", "") == ""
        deposit = data.get("deposit_address", "")
        assert _is_near_virtual_address(deposit), (
            "Empty origin_asset_id should still return virtual address"
        )

    def test_completely_bogus_origin_asset_falls_back_to_any_input(self):
        """A totally made-up asset ID can't be looked up in the token list for EXACT_INPUT,
        so the server falls back to ANY_INPUT mode and returns 200 with a virtual address."""
        resp = _create(
            origin_asset_id="nep141:totally-fake-token.near",
            origin_refund_address="0x0000000000000000000000000000000000000001",
            amount_usd="10.00",
        )
        assert resp.status_code == 200, resp.text
        deposit = resp.json().get("deposit_address", "")
        assert deposit, "deposit_address must be non-empty"
        # Fell back to ANY_INPUT → virtual address
        assert _is_near_virtual_address(deposit), (
            f"Expected ANY_INPUT fallback (virtual address) for unknown asset, got: {deposit!r}"
        )

    def test_origin_asset_without_amount_usd_falls_back_to_any_input(self):
        """If amount_usd is omitted we can't compute the EXACT_INPUT token amount,
        so the server falls back to ANY_INPUT and returns a virtual address."""
        if not config.ORIGIN_ASSET_ID:
            pytest.skip("ORIGIN_ASSET_ID not configured")
        payload = {
            "dest_asset_id": config.DEST_ASSET_ID,
            "dest_chain":    config.DEST_CHAIN,
            "dest_address":  config.DEST_ADDRESS,
            "origin_asset_id": config.ORIGIN_ASSET_ID,
            "expires_in_seconds": 3600,
        }
        resp = requests.post(
            f"{config.V1}/payments",
            json=payload,
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200, resp.text
        deposit = resp.json().get("deposit_address", "")
        assert deposit
        assert _is_near_virtual_address(deposit), (
            "Without amount_usd should fall back to ANY_INPUT (virtual address)"
        )


# ── chain-specific deposit address ───────────────────────────────────────────

@pytest.fixture(scope="module")
def origin_payment():
    """Single chain-specific payment shared across TestChainSpecificDepositAddress tests."""
    if not config.ORIGIN_ASSET_ID:
        pytest.skip("ORIGIN_ASSET_ID not configured")
    resp = _create(
        origin_asset_id=config.ORIGIN_ASSET_ID,
        origin_refund_address=config.ORIGIN_REFUND_ADDRESS,
    )
    assert resp.status_code == 200, f"origin_payment fixture failed: {resp.text}"
    return resp.json()


@pytest.fixture(scope="module")
def origin_payment_with_secrets():
    """Chain-specific payment with callback_url + metadata, for public-endpoint stripping test."""
    if not config.ORIGIN_ASSET_ID:
        pytest.skip("ORIGIN_ASSET_ID not configured")
    resp = _create(
        origin_asset_id=config.ORIGIN_ASSET_ID,
        origin_refund_address=config.ORIGIN_REFUND_ADDRESS,
        callback_url="https://example.com/cb",
        metadata={"secret": "do-not-expose"},
    )
    assert resp.status_code == 200, f"origin_payment_with_secrets fixture failed: {resp.text}"
    return resp.json()


class TestChainSpecificDepositAddress:

    def test_returns_200(self, origin_payment):
        assert origin_payment.get("id"), "fixture must have an id"

    def test_deposit_address_is_not_virtual(self, origin_payment):
        """Must NOT be the 64-char NEAR Intents hex - must be a real chain address."""
        deposit = origin_payment.get("deposit_address", "")
        assert deposit, "deposit_address must be non-empty"
        assert not _is_near_virtual_address(deposit), (
            f"Got a NEAR virtual address {deposit!r} - expected a native chain address"
        )

    def test_origin_asset_id_echoed_in_response(self, origin_payment):
        assert origin_payment.get("origin_asset_id") == config.ORIGIN_ASSET_ID

    def test_dest_fields_unchanged(self, origin_payment):
        """The destination fields must still match what we requested."""
        assert origin_payment["dest_asset_id"] == config.DEST_ASSET_ID
        assert origin_payment["dest_chain"]    == config.DEST_CHAIN
        assert origin_payment["dest_address"]  == config.DEST_ADDRESS

    def test_status_is_awaiting_deposit(self, origin_payment):
        assert origin_payment["status"] == "AWAITING_DEPOSIT"

    def test_payment_is_retrievable_after_creation(self, origin_payment):
        """Payment created with origin_asset_id must be readable via GET /v1/payments/{id}."""
        resp = requests.get(
            f"{config.V1}/payments/{origin_payment['id']}",
            headers=config.AUTH_HEADERS,
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200
        got = resp.json()
        assert got["id"] == origin_payment["id"]
        assert got["deposit_address"] == origin_payment["deposit_address"]
        assert got["origin_asset_id"] == config.ORIGIN_ASSET_ID

    def test_public_endpoint_exposes_deposit_address(self, origin_payment):
        """GET /v1/pay/{id} must show deposit_address so the customer can send funds."""
        resp = requests.get(
            f"{config.BASE_URL}/v1/pay/{origin_payment['id']}",
            timeout=config.REQUEST_TIMEOUT,
        )
        assert resp.status_code == 200
        pub = resp.json()
        assert pub["deposit_address"] == origin_payment["deposit_address"]

    def test_public_endpoint_strips_sensitive_fields(self, origin_payment_with_secrets):
        """callback_url and metadata must not leak through the public endpoint."""
        resp = requests.get(
            f"{config.BASE_URL}/v1/pay/{origin_payment_with_secrets['id']}",
            timeout=config.REQUEST_TIMEOUT,
        )
        pub = resp.json()
        assert not pub.get("callback_url")
        assert not pub.get("metadata")

    @needs_origin
    def test_without_refund_address_requires_refund_address(self):
        """NEAR Intents EXACT_INPUT+ORIGIN_CHAIN requires a valid refundTo address.
        Omitting origin_refund_address must return an error (502 from upstream)."""
        resp = _create(origin_asset_id=config.ORIGIN_ASSET_ID)
        assert resp.status_code in (400, 502), (
            f"Expected error when refund address is missing, got {resp.status_code}: {resp.text}"
        )

    @needs_origin
    def test_two_payments_same_origin_get_different_deposit_addresses(self):
        """Each payment must have a unique deposit address."""
        r1 = _create(
            origin_asset_id=config.ORIGIN_ASSET_ID,
            origin_refund_address=config.ORIGIN_REFUND_ADDRESS,
        )
        r2 = _create(
            origin_asset_id=config.ORIGIN_ASSET_ID,
            origin_refund_address=config.ORIGIN_REFUND_ADDRESS,
        )
        assert r1.status_code == 200, r1.text
        assert r2.status_code == 200, r2.text
        assert r1.json()["deposit_address"] != r2.json()["deposit_address"], (
            "Each payment must get a unique deposit address"
        )

    @needs_origin
    def test_deposit_address_format_matches_origin_chain(self):
        """If ORIGIN_ASSET_ID is an ETH asset, deposit address should be 0x..."""
        data = _create(
            origin_asset_id=config.ORIGIN_ASSET_ID,
            origin_refund_address=config.ORIGIN_REFUND_ADDRESS,
        ).json()
        deposit = data["deposit_address"]

        asset = config.ORIGIN_ASSET_ID.lower()
        if ":eth-" in asset or ":arb-" in asset or ":base-" in asset or ":op-" in asset:
            assert _is_evm_address(deposit), (
                f"Expected EVM 0x address for EVM origin asset, got: {deposit!r}"
            )
        elif ":sol-" in asset:
            assert _is_solana_address(deposit), (
                f"Expected Solana base58 address for SOL origin asset, got: {deposit!r}"
            )
        # For other chains we just assert it's non-virtual (already covered above)

    @needs_origin
    def test_amount_usd_affects_deposit_amount_hint(self):
        """Two payments with different amount_usd should both succeed (the amount
        feeds into the EXACT_INPUT calc - we just assert both return 200 and valid addresses)."""
        small = _create(
            origin_asset_id=config.ORIGIN_ASSET_ID,
            origin_refund_address=config.ORIGIN_REFUND_ADDRESS,
            amount_usd="5.00",
        )
        large = _create(
            origin_asset_id=config.ORIGIN_ASSET_ID,
            origin_refund_address=config.ORIGIN_REFUND_ADDRESS,
            amount_usd="500.00",
        )
        assert small.status_code == 200, small.text
        assert large.status_code == 200, large.text
        for resp in (small.json(), large.json()):
            assert not _is_near_virtual_address(resp["deposit_address"])


# ── known EVM asset IDs ───────────────────────────────────────────────────────

# These are well-known stable assets. Parameterised so each gets its own test node.
KNOWN_EVM_ASSETS = [
    ("eth_usdt",  "nep141:eth-0xdac17f958d2ee523a2206206994597c13d831ec7.omft.near"),
    ("eth_usdc",  "nep141:eth-0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48.omft.near"),
    ("base_usdc", "nep141:base-0x833589fcd6edb6e08f4c7c32d4f71b54bda02913.omft.near"),
]

# A generic ETH refund address used for the known-asset tests.
_GENERIC_ETH_REFUND = "0x0000000000000000000000000000000000000001"


@pytest.mark.parametrize("label,asset_id", KNOWN_EVM_ASSETS)
def test_known_evm_asset_returns_evm_deposit_address(label, asset_id):
    """For well-known EVM assets the deposit address must be a valid 0x address."""
    resp = _create(
        origin_asset_id=asset_id,
        origin_refund_address=_GENERIC_ETH_REFUND,
        amount_usd="10.00",
    )
    assert resp.status_code == 200, f"[{label}] {resp.text}"
    deposit = resp.json().get("deposit_address", "")
    assert _is_evm_address(deposit), (
        f"[{label}] Expected EVM address, got: {deposit!r}"
    )


@pytest.mark.parametrize("label,asset_id", KNOWN_EVM_ASSETS)
def test_known_evm_asset_not_virtual(label, asset_id):
    resp = _create(
        origin_asset_id=asset_id,
        origin_refund_address=_GENERIC_ETH_REFUND,
        amount_usd="10.00",
    )
    assert resp.status_code == 200, f"[{label}] {resp.text}"
    deposit = resp.json().get("deposit_address", "")
    assert not _is_near_virtual_address(deposit), (
        f"[{label}] Got NEAR virtual address - expected native EVM address"
    )
