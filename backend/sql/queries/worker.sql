-- name: ClaimStaleAnalyzingItems :many
WITH stale_items AS (
    SELECT item.id
    FROM items AS item
    WHERE item.status = 'ANALYZING'
      AND item.last_status_updated_at < sqlc.arg(stale_before)
    ORDER BY item.last_status_updated_at, item.id
    FOR UPDATE SKIP LOCKED
    LIMIT sqlc.arg(batch_size)
)
UPDATE items AS claimed_item
SET last_status_updated_at = NOW()
FROM stale_items
WHERE claimed_item.id = stale_items.id
RETURNING claimed_item.id;

-- name: GetItemForAnalysis :one
SELECT item.id,
       item.offer_title,
       item.offer_description,
       item.want_description,
       item.status
FROM items AS item
WHERE item.id = $1;

-- name: CompleteItemAnalysis :execrows
UPDATE items
SET offer_category_id = sqlc.arg(offer_category_id),
    want_category_id = sqlc.arg(want_category_id),
    param_richness = sqlc.arg(param_richness),
    is_category_manual = sqlc.arg(is_category_manual),
    offer_embedding_local = sqlc.arg(offer_embedding_local)::vector,
    want_embedding_local = sqlc.arg(want_embedding_local)::vector,
    status = 'MATCHING',
    last_status_updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = 'ANALYZING';
