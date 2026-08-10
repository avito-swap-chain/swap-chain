DROP INDEX IF EXISTS items_offer_embedding_local_cosine_idx;

UPDATE items
SET offer_embedding = COALESCE(offer_embedding_local, offer_embedding),
    want_embedding = COALESCE(want_embedding_local, want_embedding);

UPDATE categories
SET embedding = COALESCE(embedding_local, embedding);

ALTER TABLE items
    DROP COLUMN image_amount,
    DROP COLUMN want_embedding_local,
    DROP COLUMN offer_embedding_local;

ALTER TABLE categories
    DROP COLUMN embedding_local;
