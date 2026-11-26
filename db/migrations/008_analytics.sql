-- Recommendation impressions and user feedback for analytics.
CREATE TABLE IF NOT EXISTS recommendation_requests (
    trace_id      text PRIMARY KEY,
    client_id     text NOT NULL,
    intent        text NOT NULL DEFAULT '',
    fallback      boolean NOT NULL DEFAULT false,
    shown_count   integer NOT NULL DEFAULT 0,
    skipped_count integer NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS recommendation_events (
    id         bigserial PRIMARY KEY,
    trace_id   text NOT NULL REFERENCES recommendation_requests(trace_id) ON DELETE CASCADE,
    client_id  text NOT NULL,
    product_id text NOT NULL,
    event      text NOT NULL,
    reason     text NOT NULL DEFAULT '',
    detail     text NOT NULL DEFAULT '',
    rank       integer NOT NULL DEFAULT 0,
    score      numeric(5,4) NOT NULL DEFAULT 0,
    type       text NOT NULL DEFAULT '',
    source     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS recommendation_requests_client_time
    ON recommendation_requests (client_id, created_at DESC);
CREATE INDEX IF NOT EXISTS recommendation_events_time
    ON recommendation_events (created_at DESC);
CREATE INDEX IF NOT EXISTS recommendation_events_product_event
    ON recommendation_events (product_id, event);
CREATE INDEX IF NOT EXISTS recommendation_events_trace
    ON recommendation_events (trace_id);
