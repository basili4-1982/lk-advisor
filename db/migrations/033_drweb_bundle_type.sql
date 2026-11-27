-- The vendor page lists Desktop/Server as components of Enterprise Security
-- Suite, so a pair with the Enterprise kit is a bundle, not a generic
-- adjacent task.
UPDATE relations r
SET type = 'bundle', updated_at = now()
WHERE r.source = 'vendor'
  AND r.type = 'adjacent_task'
  AND EXISTS (
      SELECT 1 FROM lk_items i
      WHERE i.item_id IN (r.from_item_id, r.to_item_id)
        AND i.title ILIKE '%enterprise%'
  )
  AND NOT EXISTS (
      SELECT 1 FROM relations x
      WHERE x.from_item_id = r.from_item_id AND x.to_item_id = r.to_item_id
        AND x.type = 'bundle'
  );
