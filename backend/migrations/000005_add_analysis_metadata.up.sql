ALTER TABLE users
    ADD COLUMN success_rate NUMERIC(4, 3) NOT NULL DEFAULT 0
        CHECK (success_rate >= 0 AND success_rate <= 1);

ALTER TABLE items
    ADD COLUMN offer_category_id INTEGER REFERENCES categories (id),
    ADD COLUMN want_category_id INTEGER REFERENCES categories (id),
    ADD COLUMN visual_quality TEXT
        CHECK (visual_quality IN ('NEW', 'EXCELLENT', 'GOOD', 'FAIR', 'POOR')),
    ADD COLUMN quality_score NUMERIC(4, 3)
        CHECK (quality_score >= 0 AND quality_score <= 1),
    ADD COLUMN param_richness NUMERIC(4, 3)
        CHECK (param_richness >= 0 AND param_richness <= 1),
    ADD COLUMN is_category_manual BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN last_status_updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX items_status_updated_idx
    ON items (status, last_status_updated_at, id);

CREATE INDEX items_matching_categories_idx
    ON items (offer_category_id, want_category_id, id)
    WHERE status = 'MATCHING';
