-- Speed up registry-to-catalog matching by product name.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS lk_items_title_trgm
    ON lk_items USING gin (title gin_trgm_ops);
