CREATE TABLE item_wishes (
    id BIGSERIAL PRIMARY KEY,
    item_id BIGINT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    want_category_id INT REFERENCES categories(id),
    want_description TEXT NOT NULL,
    want_embedding_local vector(1024),
    want_embedding_external vector(1024)
);

ALTER TABLE items
    DROP COLUMN want_category_id,
    DROP COLUMN want_description,
    DROP COLUMN want_embedding_local,
    DROP COLUMN offer_category,
    DROP COLUMN want_category;

ALTER TABLE items
    ADD COLUMN offer_embedding_external vector(1024);

CREATE INDEX item_wishes_item_id_idx ON item_wishes(item_id);
CREATE INDEX item_wishes_want_embedding_local_cosine_idx ON item_wishes USING hnsw (want_embedding_local vector_cosine_ops);
CREATE INDEX item_wishes_want_embedding_external_cosine_idx ON item_wishes USING hnsw (want_embedding_external vector_cosine_ops);
CREATE INDEX items_offer_embedding_external_cosine_idx ON items USING hnsw (offer_embedding_external vector_cosine_ops);
