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
    DROP COLUMN IF EXISTS image_amount;

ALTER INDEX items_offer_embedding_local_cosine_idx
    RENAME TO items_offer_embedding_cosine_idx;

ALTER TABLE items
    RENAME COLUMN want_embedding_local TO want_embedding;

ALTER TABLE items
    RENAME COLUMN offer_embedding_local TO offer_embedding;

ALTER TABLE categories
    DROP COLUMN IF EXISTS is_system,
    ALTER COLUMN embedding_local SET NOT NULL;

ALTER INDEX categories_embedding_local_cosine_idx
    RENAME TO categories_embedding_cosine_idx;

ALTER TABLE categories
    RENAME COLUMN embedding_local TO embedding;
