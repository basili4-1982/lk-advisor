-- Short, human-readable names for catalog items, curated by merchandisers.
CREATE TABLE IF NOT EXISTS item_aliases (
    item_id    text PRIMARY KEY REFERENCES lk_items(item_id) ON DELETE CASCADE,
    short_name text NOT NULL,
    updated_by text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS item_aliases_short_name_idx
    ON item_aliases (lower(short_name));
