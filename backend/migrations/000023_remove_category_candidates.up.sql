ALTER TABLE item_wishes
    DROP CONSTRAINT IF EXISTS item_wishes_category_candidates_positive,
    DROP CONSTRAINT IF EXISTS item_wishes_category_candidates_limit,
    DROP COLUMN IF EXISTS category_candidate_ids;

ALTER TABLE items
    DROP CONSTRAINT IF EXISTS items_offer_category_candidates_positive,
    DROP CONSTRAINT IF EXISTS items_offer_category_candidates_limit,
    DROP COLUMN IF EXISTS offer_category_candidate_ids;
