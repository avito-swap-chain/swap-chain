BEGIN;

ALTER TABLE chat_messages
    ADD COLUMN item_id BIGINT REFERENCES items (id);

-- Старый чат был привязан к цепочке и мог одновременно описывать два ребра
-- двухстороннего обмена. Для таких legacy-сообщений выбираем меньший item ID:
-- это детерминированно сохраняет единую историю без копирования сообщений.
UPDATE chat_messages AS message
SET item_id = (
    SELECT owner.item_id
    FROM chain_items AS owner
    JOIN chain_items AS recipient
      ON recipient.chain_id = owner.chain_id
     AND recipient.next_item_id = owner.item_id
    WHERE owner.chain_id = message.chain_id
      AND (
          (owner.user_id = message.sender_user_id AND recipient.user_id = message.recipient_user_id)
          OR
          (owner.user_id = message.recipient_user_id AND recipient.user_id = message.sender_user_id)
      )
    ORDER BY owner.item_id
    LIMIT 1
);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM chat_messages WHERE item_id IS NULL) THEN
        RAISE EXCEPTION 'cannot resolve item for legacy chat message';
    END IF;
END
$$;

ALTER TABLE chat_messages
    ALTER COLUMN item_id SET NOT NULL;

-- Старый ключ идемпотентности был scoped к chain_id. Если клиент повторно
-- использовал один client_message_id в двух цепочках, сохраняем оба сообщения
-- и детерминированно переименовываем только конфликтующие legacy-ключи.
DO $$
DECLARE
    duplicate_row RECORD;
    candidate TEXT;
    suffix INTEGER;
BEGIN
    FOR duplicate_row IN
        SELECT ranked.id,
               ranked.item_id,
               ranked.sender_user_id,
               ranked.recipient_user_id
        FROM (
            SELECT message.id,
                   message.item_id,
                   message.sender_user_id,
                   message.recipient_user_id,
                   ROW_NUMBER() OVER (
                       PARTITION BY message.item_id,
                                    message.sender_user_id,
                                    message.recipient_user_id,
                                    message.client_message_id
                       ORDER BY message.id
                   ) AS duplicate_rank
            FROM chat_messages AS message
        ) AS ranked
        WHERE ranked.duplicate_rank > 1
        ORDER BY ranked.id
    LOOP
        suffix := 0;
        candidate := 'migrated:' || duplicate_row.id::text;
        WHILE EXISTS (
            SELECT 1
            FROM chat_messages AS message
            WHERE message.item_id = duplicate_row.item_id
              AND message.sender_user_id = duplicate_row.sender_user_id
              AND message.recipient_user_id = duplicate_row.recipient_user_id
              AND message.client_message_id = candidate
              AND message.id <> duplicate_row.id
        ) LOOP
            suffix := suffix + 1;
            candidate := 'migrated:' || duplicate_row.id::text || ':' || suffix::text;
        END LOOP;
        UPDATE chat_messages
        SET client_message_id = candidate
        WHERE id = duplicate_row.id;
    END LOOP;
END
$$;

ALTER TABLE chat_messages
    DROP CONSTRAINT chat_messages_chain_id_sender_user_id_recipient_user_id_cli_key,
    ADD CONSTRAINT chat_messages_item_sender_recipient_client_key
        UNIQUE (item_id, sender_user_id, recipient_user_id, client_message_id);

DROP INDEX chat_messages_sender_thread_idx;
DROP INDEX chat_messages_recipient_thread_idx;

CREATE INDEX chat_messages_sender_item_thread_idx
    ON chat_messages (item_id, sender_user_id, recipient_user_id, id);
CREATE INDEX chat_messages_recipient_item_thread_idx
    ON chat_messages (item_id, recipient_user_id, sender_user_id, id);

CREATE TABLE chat_item_read_states (
    item_id BIGINT NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users (id),
    counterpart_user_id BIGINT NOT NULL REFERENCES users (id),
    last_read_message_id BIGINT NOT NULL REFERENCES chat_messages (id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (item_id, user_id, counterpart_user_id),
    CHECK (user_id <> counterpart_user_id)
);

INSERT INTO chat_item_read_states (
    item_id,
    user_id,
    counterpart_user_id,
    last_read_message_id,
    updated_at
)
SELECT cursor.item_id,
       state.user_id,
       state.counterpart_user_id,
       MAX(state.last_read_message_id),
       MAX(state.updated_at)
FROM chat_read_states AS state
JOIN chat_messages AS cursor ON cursor.id = state.last_read_message_id
GROUP BY cursor.item_id, state.user_id, state.counterpart_user_id;

DROP TABLE chat_read_states;
ALTER TABLE chat_item_read_states RENAME TO chat_read_states;

COMMIT;
