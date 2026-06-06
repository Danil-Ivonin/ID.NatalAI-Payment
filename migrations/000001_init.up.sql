CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'RUB',
    description TEXT NOT NULL,
    product_code TEXT,
    status TEXT NOT NULL CHECK (
        status IN (
            'created',
            'waiting_for_payment',
            'succeeded',
            'failed',
            'expired',
            'cancelled',
            'refunded'
            )
        ),
    idempotency_key TEXT NOT NULL,
    paid_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payments_amount_positive CHECK (amount_minor > 0),
    CONSTRAINT payments_idempotency_unique UNIQUE (idempotency_key)
);

CREATE INDEX idx_payments_user_id ON payments(user_id);

CREATE TABLE payment_provider_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id),
    provider TEXT NOT NULL DEFAULT 'robokassa',
    provider_invoice_id BIGINT NOT NULL,
    payment_url TEXT NOT NULL,
    provider_status TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payment_provider_invoice_unique
        UNIQUE (provider, provider_invoice_id),
    CONSTRAINT payment_provider_invoice_payment_unique
        UNIQUE (payment_id, provider)
);

CREATE INDEX idx_provider_invoices_payment_id
    ON payment_provider_invoices(payment_id);

CREATE TABLE payment_provider_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    provider_invoice_id BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    raw_payload JSONB NOT NULL,
    signature_valid BOOLEAN NOT NULL DEFAULT false,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,

CONSTRAINT provider_event_unique UNIQUE (provider, payload_hash)
);

CREATE INDEX idx_provider_events_invoice_id
    ON payment_provider_events(provider, provider_invoice_id);

CREATE TABLE payment_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id),
    from_status TEXT,
    to_status TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_status_history_payment_id
    ON payment_status_history(payment_id);
