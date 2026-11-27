ALTER TABLE vendor_products
    ADD COLUMN IF NOT EXISTS category text NOT NULL DEFAULT '';
