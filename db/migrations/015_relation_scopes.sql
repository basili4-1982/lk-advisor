-- Scoped relation status and TTL: a link can work for B2B/IT and fail for B2C.
ALTER TABLE recommendation_requests
    ADD COLUMN IF NOT EXISTS industry text NOT NULL DEFAULT '';

ALTER TABLE recommendation_events
    ADD COLUMN IF NOT EXISTS industry text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS relation_scopes (
    id             bigserial PRIMARY KEY,
    from_id        text NOT NULL,
    to_id          text NOT NULL,
    type           text NOT NULL,
    audience       text NOT NULL DEFAULT '',
    industry       text NOT NULL DEFAULT '',
    status         text NOT NULL DEFAULT 'quarantine',
    confidence     numeric(4,3) NOT NULL DEFAULT 0.8,
    reviewed_by    text NOT NULL DEFAULT '',
    reviewed_at    timestamptz,
    valid_until    timestamptz,
    last_tested_at timestamptz,
    review_note    text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (from_id, to_id, type, audience, industry)
);

CREATE INDEX IF NOT EXISTS relation_scopes_lookup
    ON relation_scopes (from_id, type, to_id, audience, industry);
