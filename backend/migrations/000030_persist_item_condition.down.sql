ALTER TABLE items
    DROP CONSTRAINT IF EXISTS items_user_condition_score,
    ALTER COLUMN quality_score DROP NOT NULL,
    ALTER COLUMN quality_score DROP DEFAULT;
