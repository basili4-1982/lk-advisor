-- Hypothesis testing platform: one experiment is a testable relation/slot hypothesis.
CREATE TABLE IF NOT EXISTS experiments (
    id             text PRIMARY KEY,
    title          text NOT NULL,
    hypothesis     text NOT NULL,
    status         text NOT NULL DEFAULT 'draft',
    audience       text NOT NULL DEFAULT '',
    industry       text NOT NULL DEFAULT '',
    metric         text NOT NULL DEFAULT 'conversion',
    guardrails     text NOT NULL DEFAULT '',
    variants       jsonb NOT NULL DEFAULT '[]'::jsonb,
    traffic_split  numeric(4,3) NOT NULL DEFAULT 0.5,
    from_id        text NOT NULL DEFAULT '',
    to_id          text NOT NULL DEFAULT '',
    relation_type  text NOT NULL DEFAULT '',
    treatment      text NOT NULL DEFAULT 'force_relation',
    owner          text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    started_at     timestamptz,
    ends_at        timestamptz,
    decision       text NOT NULL DEFAULT '',
    decision_note  text NOT NULL DEFAULT '',
    decided_at     timestamptz
);

CREATE INDEX IF NOT EXISTS experiments_status_idx ON experiments (status, audience, industry);

CREATE TABLE IF NOT EXISTS experiment_assignments (
    experiment_id text NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    client_id     text NOT NULL,
    variant       text NOT NULL,
    assigned_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (experiment_id, client_id)
);

CREATE TABLE IF NOT EXISTS experiment_events (
    id            bigserial PRIMARY KEY,
    experiment_id text NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    client_id     text NOT NULL,
    variant       text NOT NULL,
    trace_id      text NOT NULL DEFAULT '',
    product_id    text NOT NULL DEFAULT '',
    event         text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS experiment_events_lookup
    ON experiment_events (experiment_id, variant, event);
CREATE INDEX IF NOT EXISTS experiment_events_trace
    ON experiment_events (trace_id, product_id, event);
