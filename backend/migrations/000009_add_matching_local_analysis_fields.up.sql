ALTER TABLE items
    ADD COLUMN offer_embedding_local vector(1024),
    ADD COLUMN want_embedding_local vector(1024),
    ADD COLUMN image_amount INTEGER NOT NULL DEFAULT 0
        CHECK (image_amount >= 0 AND image_amount <= 10);

ALTER TABLE categories
    ADD COLUMN embedding_local vector(1024);

UPDATE items
SET offer_embedding_local = offer_embedding,
    want_embedding_local = want_embedding,
    image_amount = cardinality(image_urls);

UPDATE categories
SET embedding_local = embedding;

CREATE INDEX items_offer_embedding_local_cosine_idx
    ON items USING hnsw (offer_embedding_local vector_cosine_ops);
