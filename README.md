# meowpayments

Custom crypto payment gateway built on [NEAR Intents 1-click API](https://1click.chaindefuser.com). Accepts any supported token from any chain and delivers the swapped asset to your wallet.

![Language](https://img.shields.io/badge/Go-1.23+-blue.svg)
![Status](https://img.shields.io/badge/status-active-success.svg)

---

## How it works

1. Your backend calls `POST /v1/payments` with a destination asset and address.
2. meowpayments fetches a quote from NEAR Intents and returns a deposit address.
3. The customer sends crypto to that address (any supported chain/token).
4. meowpayments polls for the swap and posts status updates to your `callback_url`.
5. You receive the swapped tokens at your destination address.

---

## Quick start

```bash
cp .env.example .env
# Fill in API_KEY, HTTP_BASE_URL, ONECLICK_API_KEY, ONECLICK_REFUND_ADDRESS,
# WORKER_WEBHOOK_SECRET, and DATABASE_URL.

docker compose up  # uses compose.yml
```

The server starts on `:8080` (or `HTTP_ADDR`). Migrations run automatically on startup.

---

## Configuration

All config is via environment variables. Copy `.env.example` for a full reference.

| Variable | Required | Description |
|---|---|---|
| `HTTP_BASE_URL` | yes | Public URL of your deployment (used in `payment_url` links) |
| `DATABASE_URL` | yes | PostgreSQL DSN |
| `API_KEY` | yes | Operator secret — protect all `/v1/payments/*` endpoints. Generate: `openssl rand -hex 32` |
| `ONECLICK_API_KEY` | yes | JWT from [partners.near-intents.org](https://partners.near-intents.org/home). Without it a 0.2% platform fee applies. |
| `ONECLICK_REFUND_ADDRESS` | yes | NEAR account or EVM address that receives funds on failed swaps |
| `WORKER_WEBHOOK_SECRET` | yes | HMAC-SHA256 signing key for callbacks. Generate: `openssl rand -hex 32` |

---

## API

Accessible through http://localhost:3434/swagger/index.html

---

## Examples

A minimal FastAPI shop demonstrating full integration lives in [`examples/shop/`](examples/shop/).

```bash
cd examples/shop
cp .env.example .env   # set meowpayments_BASE_URL, meowpayments_API_KEY, DEST_ADDRESS, etc.
pip install -r requirements.txt
uvicorn main:app --port 4234
```

Covers: checkout flow, webhook verification, tx hash submission, real-time status via WebSocket.

---

## Tests

Integration tests against a live server. Requires a running meowpayments instance.

```bash
cd tests
cp .env.example .env   # set BASE_URL, API_KEY, DEST_ASSET_ID, DEST_ADDRESS, etc.
pip install -r requirements.txt

python -m tests                        # all tests
python -m tests -k test_health         # single test
python -m tests -m "not ws"            # skip WebSocket tests
python -m tests test_payments_create.py
```

The suite checks health, auth, token listing, payment CRUD, cancellation, event log, deposit submission, origin asset flows, and WebSockets.

---

## Building from source

```bash
go build -o meowpayments ./cmd/server
./meowpayments
```