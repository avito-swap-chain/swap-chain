ALTER TABLE categories
    RENAME COLUMN embedding TO embedding_local;

ALTER INDEX categories_embedding_cosine_idx
    RENAME TO categories_embedding_local_cosine_idx;

ALTER TABLE categories
    ALTER COLUMN embedding_local DROP NOT NULL,
    ADD COLUMN is_system BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE items
    RENAME COLUMN offer_embedding TO offer_embedding_local;

ALTER TABLE items
    RENAME COLUMN want_embedding TO want_embedding_local;

ALTER INDEX items_offer_embedding_cosine_idx
    RENAME TO items_offer_embedding_local_cosine_idx;

ALTER TABLE items
    ADD COLUMN image_amount INTEGER
        GENERATED ALWAYS AS (cardinality(image_urls)) STORED;

ALTER TABLE items
    ADD CONSTRAINT items_image_amount_range
        CHECK (image_amount BETWEEN 0 AND 10);

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
