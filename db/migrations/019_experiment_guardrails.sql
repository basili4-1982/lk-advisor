-- Guardrails for the hypothesis platform: a treatment must not silently kill
-- renewal/upsell or increase fallback.
ALTER TABLE experiment_events
    ADD COLUMN IF NOT EXISTS renewal_shown integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS upsell_shown  integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS fallback      boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS experiment_events_guardrails
    ON experiment_events (experiment_id, variant, event);
