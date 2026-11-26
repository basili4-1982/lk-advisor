-- Relation-level feedback: which concrete link produced the impression.
ALTER TABLE recommendation_events
    ADD COLUMN IF NOT EXISTS candidate_id      text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_product_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS relation_type     text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS recommendation_events_candidate
    ON recommendation_events (candidate_id, event);
