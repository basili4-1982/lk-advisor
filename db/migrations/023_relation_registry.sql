-- Relation registry v2: the edge becomes an entity with provenance, scope and lifecycle.
DO $$
BEGIN
    IF to_regclass('lk_relation_candidates') IS NOT NULL
       AND to_regclass('relations') IS NULL THEN
        ALTER TABLE lk_relation_candidates RENAME TO relations;
        -- Освобождаем имя sequence: иначе повторный запуск 013 упадёт на bigserial.
        IF to_regclass('lk_relation_candidates_id_seq') IS NOT NULL
           AND to_regclass('relations_id_seq') IS NULL THEN
            ALTER SEQUENCE lk_relation_candidates_id_seq RENAME TO relations_id_seq;
        END IF;
    END IF;
    -- 013 выполняется при каждом старте и снова создаёт пустую старую таблицу.
    IF to_regclass('lk_relation_candidates') IS NOT NULL
       AND to_regclass('relations') IS NOT NULL THEN
        DROP TABLE lk_relation_candidates;
    END IF;
END $$;

ALTER TABLE relations
    ADD COLUMN IF NOT EXISTS provenance_kind  text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS owner_id         text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS version          integer NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS position         integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS quantity         numeric,
    ADD COLUMN IF NOT EXISTS metadata         jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS audience         text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS industry         text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS org_type         text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS valid_from       timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS valid_until      timestamptz,
    ADD COLUMN IF NOT EXISTS revalidate_after timestamptz,
    ADD COLUMN IF NOT EXISTS supersedes       bigint,
    ADD COLUMN IF NOT EXISTS rejection_reason text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS rejected_until   timestamptz,
    ADD COLUMN IF NOT EXISTS created_by       text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS approved_by      text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS approved_at      timestamptz,
    ADD COLUMN IF NOT EXISTS updated_at       timestamptz NOT NULL DEFAULT now();

ALTER TABLE relations ALTER COLUMN status SET DEFAULT 'candidate';

UPDATE relations SET provenance_kind = source WHERE provenance_kind = '';
UPDATE relations SET status = 'candidate' WHERE status = 'quarantine';
UPDATE relations SET status = 'active'    WHERE status = 'confirmed';

CREATE INDEX IF NOT EXISTS relations_status_updated_idx
    ON relations (status, updated_at DESC);
CREATE INDEX IF NOT EXISTS relations_type_idx ON relations (type);
CREATE INDEX IF NOT EXISTS relations_expiry_idx
    ON relations (valid_until) WHERE valid_until IS NOT NULL;

ALTER TABLE relation_types
    ADD COLUMN IF NOT EXISTS cardinality       text NOT NULL DEFAULT 'N:M',
    ADD COLUMN IF NOT EXISTS is_transitive     boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS default_ttl_days  integer NOT NULL DEFAULT 180,
    ADD COLUMN IF NOT EXISTS revalidate_on     text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS required_evidence text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS approval_role     text NOT NULL DEFAULT 'merchandiser';

UPDATE relation_types SET is_transitive = false
    WHERE code IN ('alternative', 'compatibility', 'substitute', 'same_task');
UPDATE relation_types SET default_ttl_days = 365 WHERE code IN ('accessory', 'bundle');
UPDATE relation_types SET default_ttl_days = 90  WHERE code = 'adjacent_task';
UPDATE relation_types SET required_evidence = ARRAY['spec', 'manual']
    WHERE code IN ('compatibility', 'bundle');
UPDATE relation_types SET approval_role = 'steward'
    WHERE code IN ('upgrade', 'bundle', 'compatibility');

INSERT INTO relation_types
    (code, title, description, examples, behavior, direction,
     default_confidence, status, builtin)
VALUES
    ('variant_of', 'Вариант',
     'Тот же продукт с другим сроком, количеством мест, регионом или исполнением. Структурная связь, не смысловая.',
     'Business Standard 12 мес → Business Standard 24 мес; 5 мест → 10 мест.',
     'info', 'forward', 0.99, 'active', true)
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS relation_events (
    id             bigserial PRIMARY KEY,
    relation_id    bigint NOT NULL,
    actor_kind     text NOT NULL DEFAULT 'human',
    actor_id       text NOT NULL DEFAULT '',
    action         text NOT NULL,
    payload_before jsonb,
    payload_after  jsonb,
    rubric_version text NOT NULL DEFAULT '',
    request_id     text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS relation_events_relation_idx
    ON relation_events (relation_id, created_at DESC);
