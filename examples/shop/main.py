"""
Minimal FastAPI shop example demonstrating full meowpayments integration
"""

import json
import os
import uuid
from typing import Any

import httpx

from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates

load_dotenv()

meowpayments_BASE_URL = os.environ.get("meowpayments_BASE_URL", "http://localhost:3434")
meowpayments_API_KEY = os.environ["meowpayments_API_KEY"]
# Address that will receive the swapped tokens after each payment
DEST_ADDRESS = os.environ["DEST_ADDRESS"]
# Chain + asset you want to receive (must match a token from GET /v1/tokens)
DEST_CHAIN = os.environ.get("DEST_CHAIN", "near")
DEST_ASSET_ID = os.environ.get("DEST_ASSET_ID", "nep141:usdc.near")
# Public URL of this shop (used for callback_url and payment_url display)
SHOP_BASE_URL = os.environ.get("SHOP_BASE_URL", "http://localhost:4234").rstrip("/")
WEBHOOK_SECRET = os.environ.get("WEBHOOK_SECRET", "")

app = FastAPI(title="Crypto Shop Example")
print(f"[config] SHOP_BASE_URL={SHOP_BASE_URL!r}  callback_url will be {SHOP_BASE_URL}/webhook")
if not WEBHOOK_SECRET:
    print("[config] WARNING: WEBHOOK_SECRET not set - webhook endpoint is unauthenticated!")
templates = Jinja2Templates(directory="templates")


# 
# Fake product catalogue
# 

PRODUCTS: dict[str, dict[str, Any]] = {
    "prod_001": {
        "id": "prod_001",
        "name": "Vintage Mechanical Keyboard",
        "description": "Tactile 65% layout, lubed switches, PBT keycaps.",
        "price_usd": "149.00",
        "image_icon": "fa-keyboard",
    },
    "prod_002": {
        "id": "prod_002",
        "name": "Minimal Desk Lamp",
        "description": "Warm 2700K light, USB-C powered, touch dimmer.",
        "price_usd": "59.00",
        "image_icon": "fa-lightbulb",
    },
    "prod_003": {
        "id": "prod_003",
        "name": "Noise Cancelling Headphones",
        "description": "40h battery, hybrid ANC, foldable.",
        "price_usd": "229.00",
        "image_icon": "fa-headphones",
    },
    "prod_004": {
        "id": "prod_004",
        "name": "Mechanical Wrist Rest",
        "description": "Memory foam, magnetic attachment, full-grain leather.",
        "price_usd": "39.00",
        "image_icon": "fa-hand",
    },
    "prod_005": {
        "id": "prod_005",
        "name": "USB-C Hub Pro",
        "description": "10-in-1: HDMI 4K, SD, 100W PD, Ethernet.",
        "price_usd": "79.00",
        "image_icon": "fa-plug",
    },
    "prod_006": {
        "id": "prod_006",
        "name": "Desk Cable Organiser",
        "description": "Magnetic silicone clips, pack of 6.",
        "price_usd": "19.00",
        "image_icon": "fa-cable-car",
    },
    "prod_test": {
        "id": "prod_test",
        "name": "Test Item",
        "description": "$1 item for integration testing.",
        "price_usd": "1.00",
        "image_icon": "fa-flask",
    },
}

# In-memory order store (use a real DB in production)
ORDERS: dict[str, dict[str, Any]] = {}


# 
# Helpers
# 

def meowpayments_headers() -> dict[str, str]:
    return {"X-API-Key": meowpayments_API_KEY, "Content-Type": "application/json"}


def total_usd(items: list[dict[str, Any]]) -> str:
    total = sum(float(PRODUCTS[i["product_id"]]["price_usd"]) * i["qty"] for i in items)
    return f"{total:.2f}"


# 
# Pages
# 

@app.get("/", response_class=HTMLResponse)
async def index(request: Request):
    return templates.TemplateResponse(
        "index.html",
        {"request": request, "products": list(PRODUCTS.values())},
    )


@app.get("/checkout", response_class=HTMLResponse)
async def checkout_page(request: Request):
    return templates.TemplateResponse("checkout.html", {"request": request})


@app.get("/payment/{order_id}", response_class=HTMLResponse)
async def payment_page(request: Request, order_id: str):
    order = ORDERS.get(order_id)
    if not order:
        raise HTTPException(status_code=404, detail="Order not found")
    return templates.TemplateResponse(
        "payment.html", {"request": request, "order": order, "payment": order["payment"]}
    )


# 
# API
# 

@app.get("/api/products")
async def list_products():
    return list(PRODUCTS.values())


@app.get("/api/tokens")
async def list_tokens():
    """Proxy GET /v1/tokens from meowpayments (public endpoint)."""
    async with httpx.AsyncClient() as client:
        resp = await client.get(f"{meowpayments_BASE_URL}/v1/tokens", timeout=10)
    if resp.status_code != 200:
        raise HTTPException(status_code=502, detail="Could not fetch token list")
    return resp.json()


@app.post("/api/checkout")
async def create_checkout(request: Request):
    """
    Accepts { items: [{product_id, qty}, ...], customer_email?: string }
    Creates a meowpayments payment and returns the order with deposit details.
    """
    body = await request.json()
    items: list[dict] = body.get("items", [])
    customer_email: str = body.get("customer_email", "")
    origin_asset_id: str = body.get("origin_asset_id", "")
    origin_refund_address: str = body.get("origin_refund_address", "")

    if not items:
        raise HTTPException(status_code=400, detail="Cart is empty")

    # Validate all product IDs
    for item in items:
        if item["product_id"] not in PRODUCTS:
            raise HTTPException(status_code=400, detail=f"Unknown product: {item['product_id']}")

    order_id = str(uuid.uuid4())
    amount = total_usd(items)

    # Build line items for metadata
    line_items = [
        {
            "product_id": i["product_id"],
            "name": PRODUCTS[i["product_id"]]["name"],
            "qty": i["qty"],
            "unit_price_usd": PRODUCTS[i["product_id"]]["price_usd"],
        }
        for i in items
    ]

    # Call meowpayments to create the payment
    payload = {
        "dest_asset_id": DEST_ASSET_ID,
        "dest_chain": DEST_CHAIN,
        "dest_address": DEST_ADDRESS,
        "amount_usd": amount,
        "expires_in_seconds": 3600,
        "callback_url": f"{SHOP_BASE_URL}/webhook",
        "customer_email": customer_email,
        "origin_asset_id": origin_asset_id,
        "origin_refund_address": origin_refund_address,
        "metadata": {
            "order_id": order_id,
            "line_items": line_items,
        },
    }

    async with httpx.AsyncClient() as client:
        resp = await client.post(
            f"{meowpayments_BASE_URL}/v1/payments",
            json=payload,
            headers=meowpayments_headers(),
            timeout=15,
        )

    if resp.status_code != 200:
        raise HTTPException(status_code=502, detail=f"meowpayments error: {resp.text}")

    payment = resp.json()

    payment_id = payment["id"]
    order = {
        "id": payment_id,
        "order_ref": order_id,  # internal reference for your own DB
        "line_items": line_items,
        "amount_usd": amount,
        "customer_email": customer_email,
        "payment": payment,
    }
    ORDERS[payment_id] = order

    return order


@app.get("/api/orders/{order_id}")
async def get_order(order_id: str):
    order = ORDERS.get(order_id)
    if not order:
        raise HTTPException(status_code=404, detail="Order not found")
    return order


# 
# Webhook - meowpayments calls this on every payment status change
# 

@app.post("/api/orders/{order_id}/submit")
async def submit_tx_hash(order_id: str, request: Request):
    """Customer submits their tx hash to speed up deposit detection."""
    order = ORDERS.get(order_id)
    if not order:
        raise HTTPException(status_code=404, detail="Order not found")

    body = await request.json()
    tx_hash: str = body.get("tx_hash", "").strip()
    if not tx_hash:
        raise HTTPException(status_code=400, detail="tx_hash required")

    deposit_address = order["payment"].get("deposit_address", "")
    if not deposit_address:
        raise HTTPException(status_code=400, detail="No deposit address on this payment")

    async with httpx.AsyncClient() as client:
        resp = await client.post(
            f"{meowpayments_BASE_URL}/v1/pay/{order['payment']['id']}/submit",
            json={"tx_hash": tx_hash},
            timeout=10,
        )

    return JSONResponse({"ok": resp.status_code == 200})


@app.post("/webhook")
async def webhook(request: Request):
    import hmac as _hmac, hashlib

    body = await request.body()

    # Verify HMAC signature if a secret is configured.
    if WEBHOOK_SECRET:
        sig_header = request.headers.get("X-Signature-SHA256", "")
        expected = _hmac.new(WEBHOOK_SECRET.encode(), body, hashlib.sha256).hexdigest()
        if not sig_header or not _hmac.compare_digest(expected, sig_header):
            raise HTTPException(status_code=401, detail="Invalid webhook signature")

    data = json.loads(body)
    payment_id = data.get("payment_id") or data.get("id")
    status = data.get("status")
    print(f"[webhook] received payment_id={payment_id} status={status}")

    for order in ORDERS.values():
        if order["payment"]["id"] == payment_id:
            order["payment"]["status"] = status
            print(f"[webhook] updated order {order['id']} → {status}")
            break
    else:
        print(f"[webhook] WARNING: no order found for payment_id={payment_id}")

    return JSONResponse({"ok": True})
