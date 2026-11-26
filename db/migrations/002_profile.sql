-- Sharded profile materialized views and refresh queue.

DO $$
DECLARE
    i integer;
BEGIN
    FOR i IN 0..7 LOOP
        EXECUTE format('DROP MATERIALIZED VIEW IF EXISTS client_profile_s%s', i);
        EXECUTE format($f$
            CREATE MATERIALIZED VIEW client_profile_s%s AS
            SELECT c.id AS client_id,
                   jsonb_build_object(
                       'segment', c.segment,
                       'industry', c.industry,
                       'size', c.size,
                       'licenses', COALESCE((
                           SELECT jsonb_agg(jsonb_build_object(
                               'id', l.id,
                               'product_id', l.product_id,
                               'seats', l.seats,
                               'start', l.start_date::timestamptz,
                               'end', l.end_date::timestamptz,
                               'status', l.status))
                           FROM licenses l WHERE l.client_id = c.id), '[]'::jsonb),
                       'purchases', COALESCE((
                           SELECT jsonb_agg(jsonb_build_object(
                               'id', p.id,
                               'product_id', p.product_id,
                               'date', p.date::timestamptz,
                               'amount', p.amount,
                               'quantity', p.quantity,
                               'seats', p.seats))
                           FROM purchases p WHERE p.client_id = c.id), '[]'::jsonb),
                       'environment', COALESCE((
                           SELECT to_jsonb(e) - 'client_id'
                           FROM client_environment e WHERE e.client_id = c.id), '{}'::jsonb)
                   ) AS facts
            FROM clients c
            WHERE c.shard = %s
        $f$, i, i);

        EXECUTE format(
            'CREATE UNIQUE INDEX IF NOT EXISTS client_profile_s%s_client_id ON client_profile_s%s (client_id)',
            i, i);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION enqueue_profile_refresh() RETURNS trigger AS $$
BEGIN
    INSERT INTO profile_refresh_queue (shard_id)
    SELECT shard FROM clients WHERE id = NEW.client_id
    ON CONFLICT (shard_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS purchases_profile_refresh ON purchases;
CREATE TRIGGER purchases_profile_refresh
    AFTER INSERT OR UPDATE ON purchases
    FOR EACH ROW EXECUTE FUNCTION enqueue_profile_refresh();

DROP TRIGGER IF EXISTS licenses_profile_refresh ON licenses;
CREATE TRIGGER licenses_profile_refresh
    AFTER INSERT OR UPDATE ON licenses
    FOR EACH ROW EXECUTE FUNCTION enqueue_profile_refresh();

DROP TRIGGER IF EXISTS clients_profile_refresh ON clients;
