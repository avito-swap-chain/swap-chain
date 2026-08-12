DROP INDEX IF EXISTS categories_embedding_external_cosine_idx;

ALTER TABLE categories
    DROP COLUMN embedding_external;
