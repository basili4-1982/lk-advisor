-- Demo-only audit of portfolio changes made from the cabinet page.
CREATE TABLE IF NOT EXISTS demo_audit (
    id         bigserial PRIMARY KEY,
    at         timestamptz NOT NULL DEFAULT now(),
    client_id  text NOT NULL,
    product_id text NOT NULL,
    title      text NOT NULL,
    action     text NOT NULL
);

CREATE INDEX IF NOT EXISTS demo_audit_client_idx ON demo_audit (client_id, at DESC);
