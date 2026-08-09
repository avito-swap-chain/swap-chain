-- name: FindSimilarItems :many
SELECT candidate.id,
       candidate.user_id,
       candidate.offer_title,
       candidate.offer_description,
       candidate.want_description,
       candidate.offer_embedding::vector AS offer_embedding,
       candidate.want_embedding::vector AS want_embedding,
       cardinality(candidate.image_urls)::int AS image_amount,
       COALESCE(candidate.param_richness, 0)::float8 AS param_richness,
       COALESCE(candidate.quality_score, 0)::float8 AS quality_score,
       owner.rating::float8 AS user_rating,
       owner.success_rate::float8 AS user_success_rate,
       (1.0 - (candidate.offer_embedding <=> source.want_embedding))::float8 AS similarity
FROM items AS candidate
JOIN users AS owner ON owner.id = candidate.user_id
JOIN items AS source ON source.id = $1
WHERE candidate.id != source.id
  AND candidate.user_id != source.user_id
  AND candidate.status = 'MATCHING'
  AND source.status = 'MATCHING'
  AND candidate.offer_embedding IS NOT NULL
  AND candidate.want_embedding IS NOT NULL
  AND source.offer_embedding IS NOT NULL
  AND source.want_embedding IS NOT NULL
  AND (candidate.offer_category_id = source.want_category_id
       OR candidate.offer_category_id IS NULL
       OR source.want_category_id IS NULL)
ORDER BY candidate.offer_embedding <=> source.want_embedding, candidate.id
LIMIT $2;

-- name: GetMatchingSourceItem :one
SELECT id
FROM items
WHERE id = $1
  AND status = 'MATCHING'
  AND offer_embedding IS NOT NULL
  AND want_embedding IS NOT NULL;
