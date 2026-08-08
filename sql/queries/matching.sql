-- name: GetTopK :many
SELECT i1.id, i1.user_id, i1.offer_title, i1.offer_description, i1.want_description,
       i1.offer_embedding::vector AS offer_embedding, 
       i1.want_embedding::vector AS want_embedding, 
       (1.0 - (i1.offer_embedding <=> (SELECT i2.want_embedding FROM items i2 WHERE i2.id = $1)))::float8 AS similarity
FROM items i1
WHERE i1.id != $1
ORDER BY i1.offer_embedding <=> (SELECT i2.want_embedding FROM items i2 WHERE i2.id = $1)
LIMIT $2;
