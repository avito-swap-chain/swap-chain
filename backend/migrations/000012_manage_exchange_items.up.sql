ALTER TYPE item_status ADD VALUE IF NOT EXISTS 'WITHDRAWN';

ALTER TABLE items
    ADD COLUMN analysis_version BIGINT NOT NULL DEFAULT 1
        CHECK (analysis_version > 0);
