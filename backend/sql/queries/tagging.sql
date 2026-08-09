-- name: FindCategory :many
SELECT category.id,
       category.name,
       (1.0 - (category.embedding_local <=> sqlc.arg(embedding_local)::vector))::float8 AS similarity
FROM categories AS category
WHERE category.embedding_local IS NOT NULL
  AND category.id != sqlc.arg(undefined_category_id)
ORDER BY category.embedding_local <=> sqlc.arg(embedding_local)::vector
LIMIT 2;
