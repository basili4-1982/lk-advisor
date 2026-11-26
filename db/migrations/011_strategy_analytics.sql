-- Strategy and weight dimensions for analytics and auto-A/B.
ALTER TABLE recommendation_requests
    ADD COLUMN IF NOT EXISTS strategy        text NOT NULL DEFAULT 'default',
    ADD COLUMN IF NOT EXISTS audience        text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS weights_version text NOT NULL DEFAULT 'default';

ALTER TABLE recommendation_events
    ADD COLUMN IF NOT EXISTS strategy        text NOT NULL DEFAULT 'default',
    ADD COLUMN IF NOT EXISTS audience        text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS weights_version text NOT NULL DEFAULT 'default';

CREATE INDEX IF NOT EXISTS recommendation_events_strategy
    ON recommendation_events (audience, strategy, event);

CREATE TABLE IF NOT EXISTS strategy_state (
    audience   text NOT NULL,
    strategy   text NOT NULL,
    enabled    boolean NOT NULL DEFAULT true,
    alpha      numeric(10,2) NOT NULL DEFAULT 1,
    beta       numeric(10,2) NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (audience, strategy)
);

INSERT INTO strategy_state (audience, strategy, enabled, alpha, beta)
VALUES
    ('b2b', 'hybrid',  true, 3, 2),
    ('b2b', 'popular', true, 2, 3),
    ('b2c', 'popular', true, 3, 2),
    ('b2c', 'hybrid',  true, 2, 3)
ON CONFLICT (audience, strategy) DO NOTHING;

INSERT INTO settings (key, value)
VALUES ('auto_strategy', 'false'::jsonb)
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value)
SELECT 'scoring_weights_b2b', value FROM settings WHERE key = 'scoring_weights'
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value)
SELECT 'scoring_weights_b2c', value FROM settings WHERE key = 'scoring_weights'
ON CONFLICT (key) DO NOTHING;
