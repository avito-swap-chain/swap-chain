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
       item.status,
       item.analysis_version
FROM items AS item
WHERE item.id = $1;

-- name: GetItemWishesForAnalysis :many
SELECT id, item_id, want_description
FROM item_wishes
WHERE item_id = $1;

-- name: UpdateItemWishAnalysis :exec
UPDATE item_wishes
SET want_category_id = sqlc.arg(want_category_id),
    want_embedding_local = sqlc.arg(want_embedding_local)::vector,
    want_embedding_external = sqlc.arg(want_embedding_external)::vector
WHERE id = $1;

-- name: CompleteItemAnalysis :execrows
WITH analyzed_item AS (
    UPDATE items
    SET offer_category_id = sqlc.arg(offer_category_id),
        param_richness = sqlc.arg(param_richness),
        is_category_manual = sqlc.arg(is_category_manual),
        offer_embedding_local = sqlc.arg(offer_embedding_local)::vector,
        offer_embedding_external = sqlc.arg(offer_embedding_external)::vector,
        status = 'MATCHING',
        last_status_updated_at = NOW()
    WHERE id = sqlc.arg(id)
      AND status = 'ANALYZING'
      AND analysis_version = sqlc.arg(analysis_version)
    RETURNING id
)
INSERT INTO matching_jobs (
    item_id,
    status,
    attempts,
    available_at,
    locked_at,
    last_error,
    updated_at
)
SELECT id, 'PENDING', 0, NOW(), NULL, NULL, NOW()
FROM analyzed_item
ON CONFLICT (item_id) DO UPDATE
SET status = 'PENDING',
    attempts = 0,
    available_at = NOW(),
    locked_at = NULL,
    last_error = NULL,
    updated_at = NOW();
