-- При откате завершённые обмены возвращаются в ACCEPTED: прежняя схема не умеет
-- хранить терминальные статусы. События получения удаляются, поскольку перевод
-- RECEIVED в IN_DELIVERY нарушил бы уникальность уже записанного события отправки.
DELETE FROM admin_delivery_events
WHERE from_status::text = 'RECEIVED'
   OR to_status::text = 'RECEIVED';

UPDATE chain_items
SET delivery_status = 'IN_DELIVERY'::delivery_status,
    delivery_updated_at = now()
WHERE delivery_status::text = 'RECEIVED';

UPDATE chains
SET status = 'ACCEPTED'::chain_status,
    updated_at = now()
WHERE status::text = 'COMPLETED';

DROP INDEX IF EXISTS chains_pending_expiration_idx;
DROP INDEX IF EXISTS chain_items_delivery_queue_idx;
ALTER TABLE admin_delivery_events
    DROP CONSTRAINT IF EXISTS admin_delivery_events_chain_item_id_to_status_key,
    DROP CONSTRAINT IF EXISTS admin_delivery_events_check;

ALTER TABLE chains ALTER COLUMN status DROP DEFAULT;
ALTER TABLE chain_items ALTER COLUMN delivery_status DROP DEFAULT;

ALTER TYPE chain_status RENAME TO chain_status_with_completed;
CREATE TYPE chain_status AS ENUM ('PENDING', 'ACCEPTED', 'REJECTED');
ALTER TABLE chains
    ALTER COLUMN status TYPE chain_status
    USING status::text::chain_status,
    ALTER COLUMN status SET DEFAULT 'PENDING'::chain_status;
DROP TYPE chain_status_with_completed;

ALTER TYPE delivery_status RENAME TO delivery_status_with_received;
CREATE TYPE delivery_status AS ENUM ('AWAITING_PVZ', 'AT_PVZ', 'IN_DELIVERY');
ALTER TABLE chain_items
    ALTER COLUMN delivery_status TYPE delivery_status
    USING delivery_status::text::delivery_status,
    ALTER COLUMN delivery_status SET DEFAULT 'AWAITING_PVZ'::delivery_status;
ALTER TABLE admin_delivery_events
    ALTER COLUMN from_status TYPE delivery_status
    USING from_status::text::delivery_status,
    ALTER COLUMN to_status TYPE delivery_status
    USING to_status::text::delivery_status;
DROP TYPE delivery_status_with_received;

ALTER TABLE admin_delivery_events
    ADD CONSTRAINT admin_delivery_events_chain_item_id_to_status_key
    UNIQUE (chain_item_id, to_status),
    ADD CONSTRAINT admin_delivery_events_check
    CHECK (from_status <> to_status);

CREATE INDEX chains_pending_expiration_idx
    ON chains (expires_at)
    WHERE status = 'PENDING';

CREATE INDEX chain_items_delivery_queue_idx
    ON chain_items (delivery_status, id);
