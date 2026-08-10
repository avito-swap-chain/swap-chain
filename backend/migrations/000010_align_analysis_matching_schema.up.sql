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
    ('Электроника: смартфон, телефон, планшет, ноутбук, компьютер, монитор, телевизор, наушники, акустика, фотоаппарат, видеокамера, приставка, консоль, роутер', TRUE),
    ('Бытовая техника: холодильник, пылесос, микроволновка, чайник, утюг, кофеварка, духовка, мультиварка, фен, кондиционер, стиралка', TRUE),
    ('Дом и дача: диван, кресло, стол, стул, шкаф, кровать, матрас, посуда, светильник, ковёр, зеркало, инструмент, сад, огород', TRUE),
    ('Спорт и отдых: велосипед, лыжи, сноуборд, коньки, тренажёр, гантели, палатка, спортинвентарь, мяч, ракетка, самокат', TRUE),
    ('Книги: книга, роман, учебник, комикс, журнал, энциклопедия, словарь, литература', TRUE),
    ('Хобби и творчество: гитара, синтезатор, краски, кисти, пряжа, вышивка, коллекционирование, настолка, пазл, рукоделие', TRUE),
    ('Одежда и аксессуары: одежда, обувь, куртка, пальто, платье, рубашка, брюки, джинсы, кроссовки, сапоги, сумка, украшения', TRUE),
    ('Детские товары: игрушка, коляска, подгузники, погремушка, качели, манеж, ходунки, радионяня', TRUE),
    ('Транспорт: автомобиль, мотоцикл, скутер, шины, запчасти, двигатель, аккумулятор, багажник', TRUE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO categories (id, name, is_system)
VALUES (47, 'Другое: неизвестное, неопределённое, прочее, нестандартное', TRUE)
ON CONFLICT (name) DO UPDATE SET is_system = TRUE;

SELECT setval(
    pg_get_serial_sequence('categories', 'id'),
    GREATEST((SELECT COALESCE(MAX(id), 1) FROM categories), 47),
    TRUE
);
