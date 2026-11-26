-- Manual review fields for relations proposed by the agent or LLM.
ALTER TABLE product_relations
    ADD COLUMN IF NOT EXISTS reason      text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS evidence    jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS reviewed_by text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reviewed_at timestamptz,
    ADD COLUMN IF NOT EXISTS review_note text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS product_relations_review
    ON product_relations (status, confidence);
