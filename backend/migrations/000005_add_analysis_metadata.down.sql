DROP INDEX IF EXISTS items_matching_categories_idx;
DROP INDEX IF EXISTS items_status_updated_idx;

ALTER TABLE items
    DROP COLUMN IF EXISTS last_status_updated_at,
    DROP COLUMN IF EXISTS is_category_manual,
    DROP COLUMN IF EXISTS param_richness,
    DROP COLUMN IF EXISTS quality_score,
    DROP COLUMN IF EXISTS visual_quality,
    DROP COLUMN IF EXISTS want_category_id,
    DROP COLUMN IF EXISTS offer_category_id;

ALTER TABLE users
    DROP COLUMN IF EXISTS success_rate;
