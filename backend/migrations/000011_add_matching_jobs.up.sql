CREATE TABLE matching_jobs (
    item_id BIGINT PRIMARY KEY REFERENCES items (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'DONE')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = 'PROCESSING') = (locked_at IS NOT NULL))
);

CREATE INDEX matching_jobs_ready_idx
    ON matching_jobs (available_at, item_id)
    WHERE status = 'PENDING';

CREATE INDEX matching_jobs_stale_idx
    ON matching_jobs (locked_at, item_id)
    WHERE status = 'PROCESSING';

-- Уже проанализированные вещи тоже должны получить первый автоматический поиск
-- после выкладки worker, иначе они навсегда останутся только в ручном GET matching.
INSERT INTO matching_jobs (item_id)
SELECT item.id
FROM items AS item
WHERE item.status = 'MATCHING'
ON CONFLICT (item_id) DO NOTHING;
