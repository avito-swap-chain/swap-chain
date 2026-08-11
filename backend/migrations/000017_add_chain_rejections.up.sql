BEGIN;

CREATE TABLE chain_rejections (
    chain_id BIGINT PRIMARY KEY REFERENCES chains(id) ON DELETE CASCADE,
    reason TEXT NOT NULL CHECK (reason IN (
        'declined',
        'expired',
        'item_unavailable',
        'item_changed',
        'item_withdrawn',
        'unknown'
    )),
    actor_user_id BIGINT REFERENCES users(id),
    item_id BIGINT REFERENCES items(id) ON DELETE SET NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO chain_rejections (chain_id, reason, occurred_at)
SELECT id, 'unknown', updated_at
FROM chains
WHERE status = 'REJECTED';

CREATE INDEX chain_rejections_reason_idx ON chain_rejections (reason);

COMMIT;
