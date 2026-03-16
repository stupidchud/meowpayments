CREATE TABLE payment_events (
    id          BIGSERIAL   PRIMARY KEY,
    payment_id  UUID        NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    event_type  TEXT        NOT NULL,
    old_status  TEXT,
    new_status  TEXT,
    payload     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_events_payment_id ON payment_events (payment_id, created_at);
