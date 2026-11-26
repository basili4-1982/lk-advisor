-- Real dataset v3: items, clients, orders, co-purchase graph and relation candidates.
CREATE TABLE IF NOT EXISTS lk_items (
    item_id     text PRIMARY KEY,
    sku         text NOT NULL DEFAULT '',
    title       text NOT NULL,
    category    text NOT NULL DEFAULT 'other',
    subcategory text NOT NULL DEFAULT '',
    source      text NOT NULL DEFAULT 'dataset_v3'
);

CREATE TABLE IF NOT EXISTS lk_clients (
    client_id    text PRIMARY KEY,
    client_group text NOT NULL DEFAULT '',
    client_type  text NOT NULL DEFAULT '',
    industry     text NOT NULL DEFAULT '',
    segment      text NOT NULL DEFAULT '',
    lk_access    text NOT NULL DEFAULT '',
    source       text NOT NULL DEFAULT 'dataset_v3'
);

CREATE TABLE IF NOT EXISTS lk_orders (
    order_id   text NOT NULL,
    client_id  text NOT NULL,
    order_week date NOT NULL,
    item_id    text NOT NULL REFERENCES lk_items(item_id) ON DELETE CASCADE,
    PRIMARY KEY (order_id, item_id)
);

CREATE INDEX IF NOT EXISTS lk_orders_item_idx ON lk_orders (item_id);
CREATE INDEX IF NOT EXISTS lk_orders_client_idx ON lk_orders (client_id);

CREATE TABLE IF NOT EXISTS co_purchase_pairs (
    item_a     text NOT NULL REFERENCES lk_items(item_id) ON DELETE CASCADE,
    item_b     text NOT NULL REFERENCES lk_items(item_id) ON DELETE CASCADE,
    count      integer NOT NULL DEFAULT 0,
    b2b_count  integer NOT NULL DEFAULT 0,
    b2c_count  integer NOT NULL DEFAULT 0,
    source     text NOT NULL DEFAULT 'dataset_v3',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (item_a, item_b)
);

CREATE INDEX IF NOT EXISTS co_purchase_pairs_count_idx ON co_purchase_pairs (count DESC);

CREATE TABLE IF NOT EXISTS lk_relation_candidates (
    id            bigserial PRIMARY KEY,
    from_item_id  text NOT NULL REFERENCES lk_items(item_id) ON DELETE CASCADE,
    to_item_id    text NOT NULL REFERENCES lk_items(item_id) ON DELETE CASCADE,
    type          text NOT NULL,
    confidence    numeric(4,3) NOT NULL,
    status        text NOT NULL DEFAULT 'quarantine',
    reason        text NOT NULL DEFAULT '',
    evidence      jsonb NOT NULL DEFAULT '[]'::jsonb,
    source        text NOT NULL DEFAULT 'co_purchase',
    created_at    timestamptz NOT NULL DEFAULT now(),
    reviewed_by   text NOT NULL DEFAULT '',
    reviewed_at   timestamptz,
    review_note   text NOT NULL DEFAULT '',
    UNIQUE (from_item_id, to_item_id, type)
);

CREATE INDEX IF NOT EXISTS lk_relation_candidates_status_idx
    ON lk_relation_candidates (status, confidence DESC);
