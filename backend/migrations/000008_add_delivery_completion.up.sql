-- RECEIVED завершает передачу одной вещи, COMPLETED — всю цепочку после
-- получения каждой вещи.
ALTER TYPE delivery_status ADD VALUE IF NOT EXISTS 'RECEIVED';
ALTER TYPE chain_status ADD VALUE IF NOT EXISTS 'COMPLETED';
