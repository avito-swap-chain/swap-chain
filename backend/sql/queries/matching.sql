-- name: FindSimilarItems :many
SELECT candidate_item.id,
       candidate_item.user_id,
       candidate_item.offer_title,
       candidate_item.offer_description,
       candidate_item.offer_category_id,
       candidate_item.visual_quality,
       candidate_item.quality_score,
       candidate_item.param_richness,
       candidate_item.is_category_manual,
       candidate_item.image_amount,
       COALESCE(candidate_reputation.rating, candidate_owner.rating) AS user_rating,
       candidate_owner.success_rate AS user_success_rate,
       candidate_item.offer_embedding_local::vector AS offer_embedding_local,
       candidate_item.offer_embedding_external::vector AS offer_embedding_external,
       BOOL_OR(source_wish.want_category_id = sqlc.arg(undefined_category_id)::int
           OR candidate_item.offer_category_id = sqlc.arg(undefined_category_id)::int) AS uses_undefined_category,
       MAX((1.0 - COALESCE(
           candidate_item.offer_embedding_external <=> source_wish.want_embedding_external,
           candidate_item.offer_embedding_local <=> source_wish.want_embedding_local
       ))::float8) AS similarity
FROM items AS candidate_item
JOIN users AS candidate_owner ON candidate_owner.id = candidate_item.user_id
LEFT JOIN user_reputation AS candidate_reputation ON candidate_reputation.user_id = candidate_owner.id
JOIN item_wishes AS source_wish ON source_wish.item_id = $1
WHERE candidate_item.id != $1
  AND candidate_item.user_id != (SELECT user_id FROM items WHERE id = $1)
  AND NOT EXISTS (
      SELECT 1
      FROM user_blocks AS block
      WHERE (block.blocker_user_id = candidate_item.user_id
             AND block.blocked_user_id = (SELECT user_id FROM items WHERE id = $1))
         OR (block.blocker_user_id = (SELECT user_id FROM items WHERE id = $1)
             AND block.blocked_user_id = candidate_item.user_id)
  )
  AND candidate_item.status = 'MATCHING'
  AND (candidate_item.offer_category_id = source_wish.want_category_id
       OR candidate_item.offer_category_id = sqlc.arg(undefined_category_id)::int
       OR source_wish.want_category_id = sqlc.arg(undefined_category_id)::int)
  AND (candidate_item.offer_embedding_external IS NOT NULL OR candidate_item.offer_embedding_local IS NOT NULL)
  AND (source_wish.want_embedding_external IS NOT NULL OR source_wish.want_embedding_local IS NOT NULL)
GROUP BY candidate_item.id, candidate_owner.id, candidate_reputation.user_id
ORDER BY similarity DESC
LIMIT $2;

-- name: GetMatchingSourceItem :one
SELECT source_item.id,
       source_item.status
FROM items AS source_item
WHERE source_item.id = $1;
