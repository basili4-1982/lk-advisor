-- Precomputed recommendations from the offline model.
CREATE TABLE IF NOT EXISTS client_recommendations (
    client_id    text NOT NULL,
    item_id      text NOT NULL REFERENCES lk_items(item_id) ON DELETE CASCADE,
    rank         integer NOT NULL,
    score        double precision NOT NULL DEFAULT 0,
    source       text NOT NULL DEFAULT '',
    reason       text NOT NULL DEFAULT '',
    generated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (client_id, item_id)
);

CREATE INDEX IF NOT EXISTS client_recommendations_client_idx
    ON client_recommendations (client_id, rank);
