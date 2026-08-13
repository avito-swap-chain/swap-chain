ALTER TABLE item_wishes
    DROP CONSTRAINT IF EXISTS item_wishes_category_candidates_positive,
    DROP CONSTRAINT IF EXISTS item_wishes_category_candidates_limit,
    DROP COLUMN IF EXISTS category_candidate_ids,
    DROP COLUMN IF EXISTS is_category_manual;

ALTER TABLE items
    DROP CONSTRAINT IF EXISTS items_offer_category_candidates_positive,
    DROP CONSTRAINT IF EXISTS items_offer_category_candidates_limit,
    DROP COLUMN IF EXISTS offer_category_candidate_ids;

UPDATE items
SET status = 'ANALYZING'
WHERE status::text = 'ACTION_REQUIRED';

DROP INDEX IF EXISTS items_matching_categories_idx;

ALTER TABLE items ALTER COLUMN status DROP DEFAULT;
ALTER TYPE item_status RENAME TO item_status_with_action_required;
CREATE TYPE item_status AS ENUM ('ANALYZING', 'MATCHING', 'LOCKED', 'WITHDRAWN');
ALTER TABLE items
    ALTER COLUMN status TYPE item_status USING status::text::item_status,
    ALTER COLUMN status SET DEFAULT 'ANALYZING';
DROP TYPE item_status_with_action_required;
