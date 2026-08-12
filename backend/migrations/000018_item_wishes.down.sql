ALTER TABLE items
    ADD COLUMN want_category_id INT REFERENCES categories(id),
    ADD COLUMN want_description TEXT,
    ADD COLUMN want_embedding_local vector(1024),
    ADD COLUMN offer_category TEXT,
    ADD COLUMN want_category TEXT;

ALTER TABLE items
    DROP COLUMN offer_embedding_external;

DROP TABLE item_wishes;
