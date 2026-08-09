-- name: FindSimilarItems :many
SELECT candidate_item.id,
       candidate_item.user_id,
       candidate_item.offer_title,
       candidate_item.offer_description,
       candidate_item.want_description,
       candidate_item.offer_category_id,
       candidate_item.want_category_id,
       candidate_item.visual_quality,
       candidate_item.quality_score,
       candidate_item.param_richness,
       candidate_item.is_category_manual,
       candidate_item.image_amount,
       candidate_owner.rating AS user_rating,
       candidate_owner.success_rate AS user_success_rate,
       candidate_item.offer_embedding_local::vector AS offer_embedding_local,
       candidate_item.want_embedding_local::vector AS want_embedding_local,
       (1.0 - (candidate_item.offer_embedding_local <=> source_item.want_embedding_local))::float8 AS similarity
FROM items AS candidate_item
JOIN users AS candidate_owner ON candidate_owner.id = candidate_item.user_id
JOIN items AS source_item ON source_item.id = $1
WHERE candidate_item.id != source_item.id
  AND candidate_item.user_id != source_item.user_id
  AND candidate_item.status = 'MATCHING'
  AND source_item.status = 'MATCHING'
  AND candidate_item.offer_category_id = source_item.want_category_id
ORDER BY candidate_item.offer_embedding_local <=> source_item.want_embedding_local
LIMIT $2;

-- name: GetMatchingSourceItem :one
SELECT source_item.id
FROM items AS source_item
WHERE source_item.id = $1
  AND source_item.status = 'MATCHING';
