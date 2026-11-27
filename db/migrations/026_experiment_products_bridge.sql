-- Bridge catalog experiments into the recommendation catalogue.
-- The recommendation engine works on the legacy `products` table; hypotheses are
-- created on `lk_items`. Mirror the referenced items (keeping availability) so a
-- treatment arm can actually show the target product.

INSERT INTO products (id, sku, title, category, subcategory, type, price, status, audience)
SELECT i.item_id,
       'lk-' || i.item_id,
       i.title,
       i.category,
       COALESCE(i.subcategory, ''),
       'license',
       0,
       CASE WHEN i.available THEN 'active' ELSE 'discontinued' END,
       'any'
FROM lk_items i
WHERE i.item_id IN (
    SELECT from_id FROM experiments
    UNION
    SELECT to_id FROM experiments
)
ON CONFLICT DO NOTHING;
