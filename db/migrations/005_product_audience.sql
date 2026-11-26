ALTER TABLE products ADD COLUMN IF NOT EXISTS audience text NOT NULL DEFAULT 'any';
CREATE INDEX IF NOT EXISTS products_audience_idx ON products (audience);
