-- The dataset snapshot has a known date even when the daily feed does not
-- provide per-row timestamps.
UPDATE lk_items
SET feed_updated_at = '2026-11-26T00:00:00Z'
WHERE feed_updated_at IS NULL;

-- The dataset has no clients without orders. These test profiles exist only to
-- demonstrate cold start: they have an audience and an industry, no history.
INSERT INTO lk_clients (client_id, client_type, industry, source) VALUES
('COLD-CLINIC', 'ЮЛ', 'Медицинские организации', 'demo_cold_start'),
('COLD-SCHOOL', 'ЮЛ', 'Образование и наука', 'demo_cold_start'),
('COLD-RETAIL', 'ЮЛ', 'Розничные сети и оптовая торговля', 'demo_cold_start')
ON CONFLICT (client_id) DO NOTHING;
