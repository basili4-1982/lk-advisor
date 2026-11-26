-- Privacy policy for public/demo views:
--   * an item is visible in B2B if at least 3 distinct clients bought it;
--   * an item is visible in B2C if at least 5 distinct B2C clients bought it.
-- Counts below the thresholds stay internal; public APIs only show aggregates.
CREATE OR REPLACE VIEW lk_item_privacy AS
SELECT i.item_id,
       count(DISTINCT o.client_id) AS buyers,
       count(DISTINCT o.client_id) FILTER (WHERE c.client_group = 'B2C') AS b2c_buyers,
       (count(DISTINCT o.client_id) >= 3) AS public_b2b,
       (count(DISTINCT o.client_id) FILTER (WHERE c.client_group = 'B2C') >= 5) AS public_b2c
FROM lk_items i
LEFT JOIN lk_orders o ON o.item_id = i.item_id
LEFT JOIN lk_clients c ON c.client_id = o.client_id
GROUP BY i.item_id;
