-- Margin plumbing for the experiment platform.
-- cost is populated when the client provides cost data; until then cost=0
-- means "margin unavailable" and the margin guardrail stays inactive.
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS cost bigint NOT NULL DEFAULT 0;

ALTER TABLE experiment_events
    ADD COLUMN IF NOT EXISTS amount bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS margin bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS margin_available boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS experiment_events_margin
    ON experiment_events (experiment_id, variant, event);
