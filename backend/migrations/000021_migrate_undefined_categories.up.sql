UPDATE items
SET is_category_manual = TRUE
WHERE status = 'ANALYZING'
  AND offer_category_id IS NOT NULL
  AND offer_category_id <> 47;

WITH unresolved_offers AS (
    SELECT item.id
    FROM items AS item
    WHERE item.offer_category_id = 47
      AND item.status IN ('ANALYZING', 'MATCHING')
)
UPDATE items AS item
SET offer_category_id = NULL,
    is_category_manual = FALSE,
    status = 'ACTION_REQUIRED',
    last_status_updated_at = NOW(),
    updated_at = NOW()
FROM unresolved_offers
WHERE item.id = unresolved_offers.id;

WITH unresolved_wishes AS (
    SELECT wish.id,
           wish.item_id
    FROM item_wishes AS wish
    JOIN items AS item ON item.id = wish.item_id
    WHERE wish.want_category_id = 47
      AND item.status IN ('ANALYZING', 'MATCHING')
), updated_wishes AS (
    UPDATE item_wishes AS wish
    SET want_category_id = NULL,
        is_category_manual = FALSE
    FROM unresolved_wishes
    WHERE wish.id = unresolved_wishes.id
    RETURNING wish.item_id
)
UPDATE items AS item
SET status = 'ACTION_REQUIRED',
    last_status_updated_at = NOW(),
    updated_at = NOW()
WHERE item.id IN (SELECT item_id FROM updated_wishes)
  AND item.status IN ('ANALYZING', 'MATCHING');

UPDATE matching_jobs AS job
SET status = 'DONE',
    locked_at = NULL,
    last_error = NULL,
    updated_at = NOW()
FROM items AS item
WHERE item.id = job.item_id
  AND item.status = 'ACTION_REQUIRED';
