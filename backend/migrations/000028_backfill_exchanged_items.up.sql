UPDATE items AS item
SET status = 'EXCHANGED',
    updated_at = now()
WHERE item.status = 'LOCKED'
  AND EXISTS (
      SELECT 1
      FROM chain_items AS participant
      JOIN chains AS chain ON chain.id = participant.chain_id
      WHERE participant.item_id = item.id
        AND chain.status = 'COMPLETED'
  );
