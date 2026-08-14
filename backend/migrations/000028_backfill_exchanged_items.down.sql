UPDATE items
SET status = 'LOCKED',
    updated_at = now()
WHERE status = 'EXCHANGED';
