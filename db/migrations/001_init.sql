-- Core tables. PostgreSQL only.

CREATE TABLE IF NOT EXISTS clients (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    segment    text NOT NULL,
    industry   text NOT NULL,
    size       text NOT NULL,
    region     text NOT NULL,
    shard      smallint NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
    id                  text PRIMARY KEY,
    sku                 text UNIQUE NOT NULL,
    title               text NOT NULL,
    category            text NOT NULL,
    subcategory         text NOT NULL DEFAULT '',
    vendor              text NOT NULL DEFAULT '',
    type                text NOT NULL,
    price               bigint NOT NULL,
    term_months         integer NOT NULL DEFAULT 0,
    seats               integer NOT NULL DEFAULT 0,
    version             text NOT NULL DEFAULT '',
    status              text NOT NULL DEFAULT 'active',
    system_requirements jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS client_environment (
    client_id  text PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
    os         text NOT NULL DEFAULT '',
    ram_gb     integer NOT NULL DEFAULT 0,
    cpu        text NOT NULL DEFAULT '',
    gpu        text NOT NULL DEFAULT '',
    source     text NOT NULL DEFAULT 'purchase',
    confidence numeric(4,3) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS product_relations (
    from_id    text NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    to_id      text NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    type       text NOT NULL,
    direction  text NOT NULL DEFAULT 'forward',
    confidence numeric(4,3) NOT NULL DEFAULT 0,
    status     text NOT NULL DEFAULT 'confirmed',
    source     text NOT NULL DEFAULT 'dataset',
    PRIMARY KEY (from_id, to_id, type)
);

CREATE TABLE IF NOT EXISTS purchases (
    id         text PRIMARY KEY,
    client_id  text NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    product_id text NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    date       date NOT NULL,
    amount     bigint NOT NULL,
    quantity   integer NOT NULL DEFAULT 1,
    seats      integer NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS licenses (
    id         text PRIMARY KEY,
    client_id  text NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    product_id text NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    seats      integer NOT NULL DEFAULT 0,
    start_date date NOT NULL,
    end_date   date NOT NULL,
    status     text NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS orders (
    id           text PRIMARY KEY,
    client_id    text NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    status       text NOT NULL,
    created_at   timestamptz NOT NULL,
    delivered_at timestamptz,
    items        jsonb NOT NULL DEFAULT '[]'::jsonb
);

CREATE TABLE IF NOT EXISTS documents (
    id            text PRIMARY KEY,
    client_id     text NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    order_id      text REFERENCES orders(id) ON DELETE SET NULL,
    type          text NOT NULL,
    created_at    timestamptz NOT NULL,
    downloaded_at timestamptz
);

CREATE TABLE IF NOT EXISTS events (
    id        text PRIMARY KEY,
    client_id text NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    type      text NOT NULL,
    ts        timestamptz NOT NULL,
    context   jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS rules (
    id        text PRIMARY KEY,
    type      text NOT NULL,
    condition text NOT NULL,
    action    text NOT NULL
);

CREATE TABLE IF NOT EXISTS promotions (
    id         text PRIMARY KEY,
    product_id text NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    discount   numeric(5,2) NOT NULL,
    valid_from date NOT NULL,
    valid_to   date NOT NULL,
    conditions text NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS ground_truth (
    client_id        text NOT NULL,
    event            text NOT NULL DEFAULT '',
    expected_product_id text NOT NULL,
    expected_type    text NOT NULL,
    PRIMARY KEY (client_id, event, expected_product_id)
);

CREATE TABLE IF NOT EXISTS agent_sessions (
    session_id       text PRIMARY KEY,
    client_id        text NOT NULL,
    last_intent      text NOT NULL DEFAULT '',
    last_product_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS profile_refresh_queue (
    shard_id     integer PRIMARY KEY,
    requested_at timestamptz NOT NULL DEFAULT now(),
    status       text NOT NULL DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS clients_shard_idx ON clients (shard);
CREATE INDEX IF NOT EXISTS products_category_idx ON products (category);
CREATE INDEX IF NOT EXISTS product_relations_from_idx ON product_relations (from_id);
CREATE INDEX IF NOT EXISTS product_relations_to_idx ON product_relations (to_id);
CREATE INDEX IF NOT EXISTS purchases_client_idx ON purchases (client_id);
CREATE INDEX IF NOT EXISTS purchases_product_idx ON purchases (product_id);
CREATE INDEX IF NOT EXISTS licenses_client_idx ON licenses (client_id);
CREATE INDEX IF NOT EXISTS licenses_product_idx ON licenses (product_id);
CREATE INDEX IF NOT EXISTS promotions_product_idx ON promotions (product_id);
CREATE INDEX IF NOT EXISTS promotions_window_idx ON promotions (valid_from, valid_to);
