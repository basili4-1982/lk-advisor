-- v5: official product family grouping and availability.
ALTER TABLE lk_items
    ADD COLUMN IF NOT EXISTS product_group text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS available boolean NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS lk_items_product_group_idx ON lk_items (product_group);
