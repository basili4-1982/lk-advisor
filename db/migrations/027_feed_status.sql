-- Daily feed metadata: when an item row was last received from the feed.
ALTER TABLE lk_items
    ADD COLUMN IF NOT EXISTS feed_updated_at timestamptz;
