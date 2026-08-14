BEGIN;

-- До этой версии владелец вещи, от которой запускался matching, получал
-- APPROVED неявно при создании цепочки. Отличить такое согласие от решения,
-- отправленного через API, по старым данным нельзя, поэтому для безопасности
-- все подтверждения ещё не принятой цепочки запрашиваются заново.
UPDATE chain_items AS participant
SET status = 'WAITING',
    updated_at = now()
FROM chains AS chain
WHERE chain.id = participant.chain_id
  AND chain.status = 'PENDING'
  AND participant.status = 'APPROVED';

COMMIT;
