UPDATE items
SET quality_score = CASE visual_quality
    WHEN 'NEW' THEN 1.0
    WHEN 'EXCELLENT' THEN 0.8
    WHEN 'GOOD' THEN 0.8
    WHEN 'FAIR' THEN 0.6
    WHEN 'POOR' THEN 0.6
    ELSE 0.8
END;

ALTER TABLE items
    ALTER COLUMN quality_score SET DEFAULT 0.8,
    ALTER COLUMN quality_score SET NOT NULL,
    ADD CONSTRAINT items_user_condition_score
        CHECK (quality_score IN (0.6, 0.8, 1.0));
