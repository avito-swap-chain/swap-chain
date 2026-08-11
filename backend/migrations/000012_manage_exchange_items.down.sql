DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM items WHERE status::text = 'WITHDRAWN') THEN
        RAISE EXCEPTION 'cannot roll back item withdrawal while withdrawn items exist';
    END IF;
END $$;

-- The partial-index predicate stores a typed item_status constant. Rebuild it
-- around the enum replacement so PostgreSQL never compares old and new enum OIDs.
DROP INDEX IF EXISTS items_matching_categories_idx;

ALTER TABLE items ALTER COLUMN status DROP DEFAULT;
ALTER TYPE item_status RENAME TO item_status_withdrawn;
CREATE TYPE item_status AS ENUM ('ANALYZING', 'MATCHING', 'LOCKED');
ALTER TABLE items
    ALTER COLUMN status TYPE item_status USING status::text::item_status,
    ALTER COLUMN status SET DEFAULT 'ANALYZING';
DROP TYPE item_status_withdrawn;

CREATE INDEX items_matching_categories_idx
    ON items (offer_category_id, want_category_id, id)
    WHERE status = 'MATCHING';

ALTER TABLE items DROP COLUMN analysis_version;
