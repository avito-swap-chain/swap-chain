ALTER TABLE chains
    ADD COLUMN cycle_key TEXT;

UPDATE chains
SET cycle_key = 'legacy:' || id::text;

WITH starts AS (
    SELECT c.id AS chain_id, count(ci.id) AS edge_count, min(ci.item_id) AS start_item_id
    FROM chains c
    JOIN chain_items ci ON ci.chain_id = c.id
    GROUP BY c.id
), paths AS (
    SELECT starts.chain_id,
           starts.edge_count,
           starts.start_item_id,
           first_edge.next_item_id AS second_item_id,
           second_edge.next_item_id AS third_item_id,
           third_edge.next_item_id AS fourth_item_id
    FROM starts
    JOIN chain_items first_edge
      ON first_edge.chain_id = starts.chain_id
     AND first_edge.item_id = starts.start_item_id
    LEFT JOIN chain_items second_edge
      ON second_edge.chain_id = starts.chain_id
     AND second_edge.item_id = first_edge.next_item_id
    LEFT JOIN chain_items third_edge
      ON third_edge.chain_id = starts.chain_id
     AND third_edge.item_id = second_edge.next_item_id
), valid_keys AS (
    SELECT chain_id,
           CASE edge_count
               WHEN 2 THEN start_item_id::text || '>' || second_item_id::text
               WHEN 3 THEN start_item_id::text || '>' || second_item_id::text || '>' || third_item_id::text
           END AS canonical_key
    FROM paths
    WHERE (edge_count = 2 AND third_item_id = start_item_id)
       OR (edge_count = 3 AND fourth_item_id = start_item_id)
), ranked AS (
    SELECT chain_id,
           canonical_key,
           row_number() OVER (PARTITION BY canonical_key ORDER BY chain_id) AS duplicate_number
    FROM valid_keys
)
UPDATE chains
SET cycle_key = ranked.canonical_key
FROM ranked
WHERE chains.id = ranked.chain_id
  AND ranked.duplicate_number = 1;

ALTER TABLE chains
    ALTER COLUMN cycle_key SET NOT NULL,
    ADD CONSTRAINT chains_cycle_key_key UNIQUE (cycle_key);
