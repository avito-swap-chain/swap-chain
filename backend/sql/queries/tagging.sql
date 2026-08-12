-- name: FindCategory :many
SELECT category.id,
       category.name,
       (1.0 - COALESCE(
           category.embedding_external <=> sqlc.arg(embedding)::vector,
           category.embedding_local <=> sqlc.arg(embedding)::vector
       ))::float8 AS similarity
FROM categories AS category
WHERE (category.embedding_local IS NOT NULL OR category.embedding_external IS NOT NULL)
  AND category.id != sqlc.arg(undefined_category_id)
ORDER BY similarity DESC
LIMIT 2;

-- name: ListCategoriesMissingEmbedding :many
SELECT category.id, category.name
FROM categories AS category
WHERE category.embedding_local IS NULL OR category.embedding_external IS NULL
ORDER BY category.id;

-- name: SetCategoryEmbedding :execrows
UPDATE categories
SET embedding_local = sqlc.arg(embedding)::vector,
    embedding_external = sqlc.arg(embedding)::vector
WHERE id = sqlc.arg(id)
  AND (embedding_local IS NULL OR embedding_external IS NULL);

-- name: CountCategoriesMissingEmbedding :one
SELECT count(*)
FROM categories
WHERE embedding_local IS NULL OR embedding_external IS NULL;

-- name: CountCategories :one
SELECT count(*)
FROM categories;

-- name: ListCategories :many
SELECT id, name, is_system
FROM categories
WHERE id != sqlc.arg(undefined_category_id)
ORDER BY id;
