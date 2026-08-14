-- name: GetChatAccessForShare :one
SELECT COALESCE(BOOL_OR(
           owner.user_id = sqlc.arg(actor_id)
           OR recipient.user_id = sqlc.arg(actor_id)
       ), false)::boolean AS is_actor_participant,
       COALESCE(BOOL_OR(
           owner.user_id = sqlc.arg(counterpart_id)
           OR recipient.user_id = sqlc.arg(counterpart_id)
       ), false)::boolean AS is_counterpart_participant,
       COALESCE(MIN(owner.chain_id) FILTER (WHERE
           (owner.user_id = sqlc.arg(actor_id) AND recipient.user_id = sqlc.arg(counterpart_id))
           OR (owner.user_id = sqlc.arg(counterpart_id) AND recipient.user_id = sqlc.arg(actor_id))
       ), 0)::bigint AS access_chain_id
FROM items AS topic_item
LEFT JOIN chain_items AS owner ON owner.item_id = topic_item.id
LEFT JOIN chain_items AS recipient
  ON recipient.chain_id = owner.chain_id
 AND recipient.next_item_id = owner.item_id
WHERE topic_item.id = sqlc.arg(item_id)
GROUP BY topic_item.id;

-- name: InsertChatMessage :one
INSERT INTO chat_messages (
    chain_id,
    item_id,
    sender_user_id,
    recipient_user_id,
    client_message_id,
    message_text
)
VALUES (
    sqlc.arg(chain_id),
    sqlc.arg(item_id),
    sqlc.arg(sender_user_id),
    sqlc.arg(recipient_user_id),
    sqlc.arg(client_message_id),
    sqlc.arg(message_text)
)
ON CONFLICT (item_id, sender_user_id, recipient_user_id, client_message_id) DO NOTHING
RETURNING id;

-- name: GetChatMessage :one
SELECT message.id,
       message.item_id,
       message.chain_id AS origin_chain_id,
       sender.id AS sender_user_id,
       sender.username AS sender_username,
       recipient.id AS recipient_user_id,
       recipient.username AS recipient_username,
       message.client_message_id,
       message.message_text,
       message.created_at
FROM chat_messages AS message
JOIN users AS sender ON sender.id = message.sender_user_id
JOIN users AS recipient ON recipient.id = message.recipient_user_id
WHERE message.id = sqlc.arg(message_id);

-- name: GetChatMessageByClientID :one
SELECT message.id,
       message.item_id,
       message.chain_id AS origin_chain_id,
       sender.id AS sender_user_id,
       sender.username AS sender_username,
       recipient.id AS recipient_user_id,
       recipient.username AS recipient_username,
       message.client_message_id,
       message.message_text,
       message.created_at
FROM chat_messages AS message
JOIN users AS sender ON sender.id = message.sender_user_id
JOIN users AS recipient ON recipient.id = message.recipient_user_id
WHERE message.item_id = sqlc.arg(item_id)
  AND message.sender_user_id = sqlc.arg(sender_user_id)
  AND message.recipient_user_id = sqlc.arg(recipient_user_id)
  AND message.client_message_id = sqlc.arg(client_message_id);

-- name: ListChatMessages :many
SELECT message.id,
       message.item_id,
       message.chain_id AS origin_chain_id,
       sender.id AS sender_user_id,
       sender.username AS sender_username,
       recipient.id AS recipient_user_id,
       recipient.username AS recipient_username,
       message.client_message_id,
       message.message_text,
       message.created_at
FROM chat_messages AS message
JOIN users AS sender ON sender.id = message.sender_user_id
JOIN users AS recipient ON recipient.id = message.recipient_user_id
WHERE message.item_id = sqlc.arg(item_id)
  AND (
      (message.sender_user_id = sqlc.arg(actor_id) AND message.recipient_user_id = sqlc.arg(counterpart_id))
      OR
      (message.sender_user_id = sqlc.arg(counterpart_id) AND message.recipient_user_id = sqlc.arg(actor_id))
  )
  AND message.id > sqlc.arg(after_id)
ORDER BY message.id ASC
LIMIT sqlc.arg(result_limit);

-- name: ListChatThreads :many
WITH incoming_transfers AS (
    SELECT DISTINCT owner.item_id,
           owner.user_id AS counterpart_user_id
    FROM chain_items AS owner
    JOIN chain_items AS recipient
      ON recipient.chain_id = owner.chain_id
     AND recipient.next_item_id = owner.item_id
    WHERE recipient.user_id = sqlc.arg(actor_id)
), messaged_transfers AS (
    SELECT DISTINCT message.item_id,
           CASE
               WHEN message.sender_user_id = sqlc.arg(actor_id) THEN message.recipient_user_id
               ELSE message.sender_user_id
           END AS counterpart_user_id
    FROM chat_messages AS message
    WHERE message.sender_user_id = sqlc.arg(actor_id)
       OR message.recipient_user_id = sqlc.arg(actor_id)
), transfers AS (
    SELECT item_id, counterpart_user_id FROM incoming_transfers
    UNION
    SELECT item_id, counterpart_user_id FROM messaged_transfers
)
SELECT topic_item.id AS item_id,
       topic_item.offer_title AS item_title,
       COALESCE(topic_item.image_urls[1], '')::text AS item_image_url,
       counterpart.id AS counterpart_user_id,
       counterpart.username AS counterpart_username,
       COALESCE(latest.id, 0)::bigint AS last_message_id,
       COALESCE(latest.origin_chain_id, 0)::bigint AS last_origin_chain_id,
       COALESCE(latest.sender_user_id, 0)::bigint AS last_sender_user_id,
       COALESCE(latest.sender_username, '')::text AS last_sender_username,
       COALESCE(latest.recipient_user_id, 0)::bigint AS last_recipient_user_id,
       COALESCE(latest.recipient_username, '')::text AS last_recipient_username,
       COALESCE(latest.client_message_id, '')::text AS last_client_message_id,
       COALESCE(latest.message_text, '')::text AS last_message_text,
       COALESCE(latest.created_at, TIMESTAMPTZ 'epoch') AS last_message_created_at,
       COALESCE(unread.unread_count, 0)::bigint AS unread_count
FROM transfers AS thread
JOIN items AS topic_item ON topic_item.id = thread.item_id
JOIN users AS counterpart ON counterpart.id = thread.counterpart_user_id
LEFT JOIN LATERAL (
    SELECT message.id,
           message.chain_id AS origin_chain_id,
           message.sender_user_id,
           sender.username AS sender_username,
           message.recipient_user_id,
           recipient.username AS recipient_username,
           message.client_message_id,
           message.message_text,
           message.created_at
    FROM chat_messages AS message
    JOIN users AS sender ON sender.id = message.sender_user_id
    JOIN users AS recipient ON recipient.id = message.recipient_user_id
    WHERE message.item_id = thread.item_id
      AND (
          (message.sender_user_id = sqlc.arg(actor_id) AND message.recipient_user_id = thread.counterpart_user_id)
          OR
          (message.sender_user_id = thread.counterpart_user_id AND message.recipient_user_id = sqlc.arg(actor_id))
      )
    ORDER BY message.id DESC
    LIMIT 1
) AS latest ON TRUE
LEFT JOIN LATERAL (
    SELECT count(*)::bigint AS unread_count
    FROM chat_messages AS message
    WHERE message.item_id = thread.item_id
      AND message.sender_user_id = thread.counterpart_user_id
      AND message.recipient_user_id = sqlc.arg(actor_id)
      AND message.id > COALESCE((
          SELECT read_state.last_read_message_id
          FROM chat_read_states AS read_state
          WHERE read_state.item_id = thread.item_id
            AND read_state.user_id = sqlc.arg(actor_id)
            AND read_state.counterpart_user_id = thread.counterpart_user_id
      ), 0)
) AS unread ON TRUE
ORDER BY latest.created_at DESC NULLS LAST,
         topic_item.id DESC,
         counterpart.id ASC;

-- name: ChatMessageBelongsToThread :one
SELECT EXISTS (
    SELECT 1
    FROM chat_messages AS message
    WHERE message.id = sqlc.arg(message_id)
      AND message.item_id = sqlc.arg(item_id)
      AND (
          (message.sender_user_id = sqlc.arg(actor_id) AND message.recipient_user_id = sqlc.arg(counterpart_id))
          OR
          (message.sender_user_id = sqlc.arg(counterpart_id) AND message.recipient_user_id = sqlc.arg(actor_id))
      )
);

-- name: UpsertChatReadState :one
INSERT INTO chat_read_states (
    item_id,
    user_id,
    counterpart_user_id,
    last_read_message_id
)
VALUES (
    sqlc.arg(item_id),
    sqlc.arg(actor_id),
    sqlc.arg(counterpart_id),
    sqlc.arg(last_read_message_id)
)
ON CONFLICT (item_id, user_id, counterpart_user_id) DO UPDATE
SET last_read_message_id = GREATEST(chat_read_states.last_read_message_id, EXCLUDED.last_read_message_id),
    updated_at = now()
RETURNING last_read_message_id;

-- name: CountUnreadChatMessages :one
SELECT count(*)::bigint
FROM chat_messages AS message
WHERE message.item_id = sqlc.arg(item_id)
  AND message.sender_user_id = sqlc.arg(counterpart_id)
  AND message.recipient_user_id = sqlc.arg(actor_id)
  AND message.id > sqlc.arg(last_read_message_id);
