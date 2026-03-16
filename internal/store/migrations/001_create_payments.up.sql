CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE payments (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Destination: what the operator wants to receive
    dest_asset_id       TEXT        NOT NULL,
    dest_chain          TEXT        NOT NULL,
    dest_address        TEXT        NOT NULL,

    -- Expected amount (operator provides one or neither)
    amount_usd          NUMERIC(20, 8),
    amount_asset        NUMERIC(78, 0),  -- raw token units (supports very large integers)
    amount_symbol       TEXT,

    -- Deposit side: populated after quote
    deposit_address     TEXT        UNIQUE,
    deposit_memo        TEXT,
    quote_expires_at    TIMESTAMPTZ,

    -- Lifecycle
    status              TEXT        NOT NULL DEFAULT 'PENDING',
    oneclick_status     TEXT,
    failure_reason      TEXT,

    -- Config
    callback_url        TEXT,
    metadata            JSONB,
    customer_email      TEXT,
    expires_at          TIMESTAMPTZ NOT NULL,

    -- Polling bookkeeping
    last_polled_at      TIMESTAMPTZ,
    poll_failures       INT         NOT NULL DEFAULT 0,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Efficient polling: only scan non-terminal payments with a deposit address
CREATE INDEX idx_payments_active
    ON payments (last_polled_at NULLS FIRST)
    WHERE status NOT IN ('COMPLETED', 'EXPIRED', 'REFUNDED', 'FAILED')
      AND deposit_address IS NOT NULL;

-- Fast lookup by deposit address (used in status webhook)
CREATE INDEX idx_payments_deposit_address
    ON payments (deposit_address)
    WHERE deposit_address IS NOT NULL;

-- Pagination / listing
CREATE INDEX idx_payments_created_at ON payments (created_at DESC);
CREATE INDEX idx_payments_status     ON payments (status);
