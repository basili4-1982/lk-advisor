CREATE INDEX IF NOT EXISTS client_recommendations_item_idx
    ON client_recommendations (item_id);

-- Two overlapping rebuilds can leave two items with the same rank for a client.
-- Keep the lexicographically smaller item and enforce one rank per client.
DELETE FROM client_recommendations a
USING client_recommendations b
WHERE a.client_id = b.client_id
  AND a.rank = b.rank
  AND a.item_id > b.item_id;

CREATE UNIQUE INDEX IF NOT EXISTS client_recommendations_client_rank_idx
    ON client_recommendations (client_id, rank);
