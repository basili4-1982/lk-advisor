-- Platform requirements for catalog items. Empty JSON means "not verified".
ALTER TABLE lk_items
    ADD COLUMN IF NOT EXISTS system_requirements jsonb NOT NULL DEFAULT '{}'::jsonb;
