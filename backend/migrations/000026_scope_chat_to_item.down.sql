BEGIN;

CREATE TABLE chat_chain_read_states (
    chain_id BIGINT NOT NULL REFERENCES chains (id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users (id),
    counterpart_user_id BIGINT NOT NULL REFERENCES users (id),
    last_read_message_id BIGINT NOT NULL REFERENCES chat_messages (id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chain_id, user_id, counterpart_user_id),
    CHECK (user_id <> counterpart_user_id),
    FOREIGN KEY (chain_id, user_id)
        REFERENCES chain_items (chain_id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (chain_id, counterpart_user_id)
        REFERENCES chain_items (chain_id, user_id) ON DELETE CASCADE
);

INSERT INTO chat_chain_read_states (
    chain_id,
    user_id,
    counterpart_user_id,
    last_read_message_id,
    updated_at
)
SELECT message.chain_id,
       state.user_id,
       state.counterpart_user_id,
       MAX(message.id),
       MAX(state.updated_at)
FROM chat_read_states AS state
JOIN chat_messages AS message
  ON message.item_id = state.item_id
 AND message.id <= state.last_read_message_id
 AND (
     (message.sender_user_id = state.user_id AND message.recipient_user_id = state.counterpart_user_id)
     OR
     (message.sender_user_id = state.counterpart_user_id AND message.recipient_user_id = state.user_id)
 )
GROUP BY message.chain_id, state.user_id, state.counterpart_user_id;

DROP TABLE chat_read_states;
ALTER TABLE chat_chain_read_states RENAME TO chat_read_states;

ALTER TABLE chat_messages
    DROP CONSTRAINT chat_messages_item_sender_recipient_client_key,
    ADD CONSTRAINT chat_messages_chain_id_sender_user_id_recipient_user_id_cli_key
        UNIQUE (chain_id, sender_user_id, recipient_user_id, client_message_id);

DROP INDEX chat_messages_sender_item_thread_idx;
DROP INDEX chat_messages_recipient_item_thread_idx;

CREATE INDEX chat_messages_sender_thread_idx
    ON chat_messages (chain_id, sender_user_id, recipient_user_id, id);
CREATE INDEX chat_messages_recipient_thread_idx
    ON chat_messages (chain_id, recipient_user_id, sender_user_id, id);

ALTER TABLE chat_messages DROP COLUMN item_id;

COMMIT;
