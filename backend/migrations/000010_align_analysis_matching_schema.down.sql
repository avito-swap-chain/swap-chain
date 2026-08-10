DELETE FROM categories AS category
WHERE category.is_system = TRUE
  AND NOT EXISTS (
      SELECT 1
      FROM items AS item
      WHERE item.offer_category_id = category.id
         OR item.want_category_id = category.id
  );

ALTER TABLE items
    DROP CONSTRAINT IF EXISTS items_image_amount_range,
    DROP COLUMN image_amount,
    ADD COLUMN offer_embedding vector(1024),
    ADD COLUMN want_embedding vector(1024),
    ADD COLUMN image_amount INTEGER NOT NULL DEFAULT 0
        CHECK (image_amount BETWEEN 0 AND 10);

UPDATE items
SET offer_embedding = offer_embedding_local,
    want_embedding = want_embedding_local,
    image_amount = cardinality(image_urls);

CREATE INDEX items_offer_embedding_cosine_idx
    ON items USING hnsw (offer_embedding vector_cosine_ops);

ALTER TABLE categories
    ADD COLUMN embedding vector(1024);

UPDATE categories
SET embedding = embedding_local;

ALTER TABLE categories
    ALTER COLUMN embedding SET NOT NULL,
    DROP COLUMN is_system;

CREATE INDEX categories_embedding_cosine_idx
    ON categories USING hnsw (embedding vector_cosine_ops);
