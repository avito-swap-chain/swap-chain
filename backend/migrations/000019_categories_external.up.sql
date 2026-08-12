ALTER TABLE categories
    ADD COLUMN embedding_external vector(1024);

CREATE INDEX categories_embedding_external_cosine_idx ON categories USING hnsw (embedding_external vector_cosine_ops);
