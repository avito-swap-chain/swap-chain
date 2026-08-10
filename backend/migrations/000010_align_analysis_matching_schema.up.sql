DROP INDEX IF EXISTS items_offer_embedding_cosine_idx;
DROP INDEX IF EXISTS categories_embedding_cosine_idx;

ALTER TABLE items
    DROP COLUMN offer_embedding,
    DROP COLUMN want_embedding,
    DROP COLUMN image_amount;

ALTER TABLE items
    ADD COLUMN image_amount INTEGER
        GENERATED ALWAYS AS (cardinality(image_urls)) STORED,
    ADD CONSTRAINT items_image_amount_range
        CHECK (image_amount BETWEEN 0 AND 10);

ALTER TABLE categories
    ALTER COLUMN embedding_local DROP NOT NULL,
    DROP COLUMN embedding,
    ADD COLUMN is_system BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX categories_embedding_local_cosine_idx
    ON categories USING hnsw (embedding_local vector_cosine_ops);

INSERT INTO categories (name, is_system)
VALUES
    ('Электроника', TRUE),
    ('Бытовая техника', TRUE),
    ('Дом и дача', TRUE),
    ('Спорт и отдых', TRUE),
    ('Книги', TRUE),
    ('Хобби и творчество', TRUE),
    ('Одежда и аксессуары', TRUE),
    ('Детские товары', TRUE),
    ('Транспорт', TRUE),
    ('Другое', TRUE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO categories (id, name, is_system)
VALUES (47, 'Не определено', TRUE)
ON CONFLICT (name) DO UPDATE SET is_system = TRUE;

SELECT setval(
    pg_get_serial_sequence('categories', 'id'),
    GREATEST((SELECT COALESCE(MAX(id), 1) FROM categories), 47),
    TRUE
);
