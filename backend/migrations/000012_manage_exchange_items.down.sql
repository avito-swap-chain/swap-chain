DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM items WHERE status = 'WITHDRAWN') THEN
        RAISE EXCEPTION 'cannot roll back item withdrawal while withdrawn items exist';
    END IF;
END $$;

ALTER TABLE items ALTER COLUMN status DROP DEFAULT;
ALTER TYPE item_status RENAME TO item_status_withdrawn;
CREATE TYPE item_status AS ENUM ('ANALYZING', 'MATCHING', 'LOCKED');
ALTER TABLE items
    ALTER COLUMN status TYPE item_status USING status::text::item_status,
    ALTER COLUMN status SET DEFAULT 'ANALYZING';
DROP TYPE item_status_withdrawn;

ALTER TABLE items DROP COLUMN analysis_version;
