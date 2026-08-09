-- name: FindCategory :many
SELECT id, name, (1.0 - (embedding <=> sqlc.arg(embedding)::vector))::float8 AS similarity
FROM categories
ORDER BY embedding <=> sqlc.arg(embedding)::vector
LIMIT 2;
