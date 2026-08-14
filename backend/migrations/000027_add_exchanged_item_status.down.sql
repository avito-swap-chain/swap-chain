DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM items WHERE status::text = 'EXCHANGED') THEN
        RAISE EXCEPTION 'cannot remove EXCHANGED item status while exchanged items exist';
    END IF;
END
$$;

DROP INDEX IF EXISTS items_matching_categories_idx;

ALTER TABLE items ALTER COLUMN status DROP DEFAULT;
ALTER TYPE item_status RENAME TO item_status_with_exchanged;
CREATE TYPE item_status AS ENUM ('ANALYZING', 'MATCHING', 'LOCKED', 'WITHDRAWN', 'ACTION_REQUIRED');
ALTER TABLE items
    ALTER COLUMN status TYPE item_status USING status::text::item_status,
    ALTER COLUMN status SET DEFAULT 'ANALYZING';
DROP TYPE item_status_with_exchanged;
