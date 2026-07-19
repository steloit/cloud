-- T11.3: invoices — the meter frozen on the billing anchor (B3: an invoice is
-- DATA, every line expands to the usage rows behind it). One invoice per
-- (org, period) so a monthly close is idempotent (UNIQUE). Lines are jsonb
-- (models.md); each carries a usage_ref back to the B2 meter. Money is integer
-- cents (ADR-025) — the shown estimate line IS the invoice line.
CREATE TABLE invoices (
    id          text PRIMARY KEY,            -- inv_<hex>
    org_id      text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    period      text NOT NULL,               -- YYYY-MM
    status      text NOT NULL DEFAULT 'accruing'
        CHECK (status IN ('accruing', 'open', 'paid', 'failed', 'disputed')),
    total_cents bigint NOT NULL DEFAULT 0,
    lines       jsonb NOT NULL DEFAULT '[]', -- [{description, project_id, cents, usage_ref}]
    tax         jsonb,                        -- {kind, id, cents} — GST etc.
    closed_at   timestamptz,                  -- accruing → open
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, period)
);
CREATE INDEX invoices_org_idx ON invoices (org_id, period DESC);
