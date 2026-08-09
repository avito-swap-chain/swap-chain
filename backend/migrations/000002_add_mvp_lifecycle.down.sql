DROP INDEX IF EXISTS chain_items_chain_id_idx;
DROP INDEX IF EXISTS chains_pending_expiration_idx;

ALTER TABLE chains
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS expires_at;

DROP INDEX IF EXISTS items_status_id_idx;
DROP TABLE IF EXISTS categories;

UPDATE items
SET offer_embedding = COALESCE(offer_embedding, array_fill(0::real, ARRAY[1024])::vector),
    want_embedding = COALESCE(want_embedding, array_fill(0::real, ARRAY[1024])::vector);

ALTER TABLE items
    DROP CONSTRAINT IF EXISTS items_image_urls_limit,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS image_urls,
    DROP COLUMN IF EXISTS status,
    ALTER COLUMN offer_embedding SET NOT NULL,
    ALTER COLUMN want_embedding SET NOT NULL;

ALTER TABLE users
    DROP COLUMN IF EXISTS created_at;

DROP TYPE IF EXISTS item_status;
