-- Runtime settings for the recommendation engine and an audit of rule changes.
CREATE TABLE IF NOT EXISTS settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO settings (key, value)
VALUES ('scoring_weights', '{"relevance":0.35,"urgency":0.25,"affinity":0.20,"margin":0.10,"confidence":0.10}'::jsonb)
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS admin_audit (
    id         bigserial PRIMARY KEY,
    actor      text NOT NULL DEFAULT 'admin',
    action     text NOT NULL,
    payload    jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
