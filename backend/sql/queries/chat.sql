-- name: GetChatAccessForShare :one
SELECT EXISTS (
           SELECT 1
           FROM chain_items AS actor
           WHERE actor.chain_id = exchange_chain.id
             AND actor.user_id = sqlc.arg(actor_id)
       ) AS is_actor_participant,
       EXISTS (
           SELECT 1
           FROM chain_items AS counterpart
           WHERE counterpart.chain_id = exchange_chain.id
             AND counterpart.user_id = sqlc.arg(counterpart_id)
       ) AS is_counterpart_participant,
       EXISTS (
           SELECT 1
           FROM chain_items AS actor
           JOIN chain_items AS counterpart
             ON counterpart.chain_id = actor.chain_id
            AND counterpart.user_id = sqlc.arg(counterpart_id)
           WHERE actor.chain_id = exchange_chain.id
             AND actor.user_id = sqlc.arg(actor_id)
             AND (
                 actor.next_item_id = counterpart.item_id
                 OR counterpart.next_item_id = actor.item_id
             )
       ) AS is_neighbor
FROM chains AS exchange_chain
WHERE exchange_chain.id = sqlc.arg(chain_id)
FOR SHARE OF exchange_chain;

-- name: InsertChatMessage :one
INSERT INTO chat_messages (
    chain_id,
    sender_user_id,
    recipient_user_id,
    client_message_id,
    message_text
)
VALUES (
    sqlc.arg(chain_id),
    sqlc.arg(sender_user_id),
    sqlc.arg(recipient_user_id),
    sqlc.arg(client_message_id),
    sqlc.arg(message_text)
)
ON CONFLICT (chain_id, sender_user_id, recipient_user_id, client_message_id) DO NOTHING
RETURNING id;

-- name: GetChatMessage :one
SELECT message.id,
       message.chain_id,
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
       message.chain_id,
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
WHERE message.chain_id = sqlc.arg(chain_id)
  AND message.sender_user_id = sqlc.arg(sender_user_id)
  AND message.recipient_user_id = sqlc.arg(recipient_user_id)
  AND message.client_message_id = sqlc.arg(client_message_id);

-- name: ListChatMessages :many
SELECT message.id,
       message.chain_id,
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
WHERE message.chain_id = sqlc.arg(chain_id)
  AND (
      (message.sender_user_id = sqlc.arg(actor_id) AND message.recipient_user_id = sqlc.arg(counterpart_id))
      OR
      (message.sender_user_id = sqlc.arg(counterpart_id) AND message.recipient_user_id = sqlc.arg(actor_id))
  )
  AND message.id > sqlc.arg(after_id)
ORDER BY message.id ASC
LIMIT sqlc.arg(result_limit);

-- name: ListChatThreads :many
WITH actor_legs AS (
    SELECT participant.chain_id,
           participant.item_id,
           participant.next_item_id
    FROM chain_items AS participant
    WHERE participant.user_id = sqlc.arg(actor_id)
),
thread_pairs AS (
    SELECT DISTINCT actor_leg.chain_id,
           counterpart.user_id AS counterpart_user_id,
           CASE
               WHEN counterpart.next_item_id = actor_leg.item_id THEN actor_leg.item_id
           END AS give_item_id,
           CASE
               WHEN actor_leg.next_item_id = counterpart.item_id THEN counterpart.item_id
           END AS receive_item_id
    FROM actor_legs AS actor_leg
    JOIN chain_items AS counterpart
      ON counterpart.chain_id = actor_leg.chain_id
     AND (
         actor_leg.next_item_id = counterpart.item_id
         OR counterpart.next_item_id = actor_leg.item_id
     )
    WHERE counterpart.user_id <> sqlc.arg(actor_id)
)
SELECT thread.chain_id,
       counterpart.id AS counterpart_user_id,
       counterpart.username AS counterpart_username,
       COALESCE(give_item.id, 0)::bigint AS give_item_id,
       COALESCE(give_item.offer_title, '')::text AS give_item_title,
       COALESCE(give_item.image_urls[1], '')::text AS give_item_image_url,
       COALESCE(receive_item.id, 0)::bigint AS receive_item_id,
       COALESCE(receive_item.offer_title, '')::text AS receive_item_title,
       COALESCE(receive_item.image_urls[1], '')::text AS receive_item_image_url,
       COALESCE(latest.id, 0)::bigint AS last_message_id,
       COALESCE(latest.sender_user_id, 0)::bigint AS last_sender_user_id,
       COALESCE(latest.sender_username, '')::text AS last_sender_username,
       COALESCE(latest.recipient_user_id, 0)::bigint AS last_recipient_user_id,
       COALESCE(latest.recipient_username, '')::text AS last_recipient_username,
       COALESCE(latest.client_message_id, '')::text AS last_client_message_id,
       COALESCE(latest.message_text, '')::text AS last_message_text,
       COALESCE(latest.created_at, TIMESTAMPTZ 'epoch') AS last_message_created_at,
       COALESCE(unread.unread_count, 0)::bigint AS unread_count
FROM thread_pairs AS thread
JOIN users AS counterpart ON counterpart.id = thread.counterpart_user_id
LEFT JOIN items AS give_item ON give_item.id = thread.give_item_id
LEFT JOIN items AS receive_item ON receive_item.id = thread.receive_item_id
LEFT JOIN LATERAL (
    SELECT message.id,
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
    WHERE message.chain_id = thread.chain_id
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
    WHERE message.chain_id = thread.chain_id
      AND message.sender_user_id = thread.counterpart_user_id
      AND message.recipient_user_id = sqlc.arg(actor_id)
      AND message.id > COALESCE((
          SELECT read_state.last_read_message_id
          FROM chat_read_states AS read_state
          WHERE read_state.chain_id = thread.chain_id
            AND read_state.user_id = sqlc.arg(actor_id)
            AND read_state.counterpart_user_id = thread.counterpart_user_id
      ), 0)
) AS unread ON TRUE
ORDER BY latest.created_at DESC NULLS LAST,
         thread.chain_id DESC,
         counterpart.id ASC;

-- name: ChatMessageBelongsToThread :one
SELECT EXISTS (
    SELECT 1
    FROM chat_messages AS message
    WHERE message.id = sqlc.arg(message_id)
      AND message.chain_id = sqlc.arg(chain_id)
      AND (
          (message.sender_user_id = sqlc.arg(actor_id) AND message.recipient_user_id = sqlc.arg(counterpart_id))
          OR
          (message.sender_user_id = sqlc.arg(counterpart_id) AND message.recipient_user_id = sqlc.arg(actor_id))
      )
);

-- name: UpsertChatReadState :one
INSERT INTO chat_read_states (
    chain_id,
    user_id,
    counterpart_user_id,
    last_read_message_id
)
VALUES (
    sqlc.arg(chain_id),
    sqlc.arg(actor_id),
    sqlc.arg(counterpart_id),
    sqlc.arg(last_read_message_id)
)
ON CONFLICT (chain_id, user_id, counterpart_user_id) DO UPDATE
SET last_read_message_id = GREATEST(chat_read_states.last_read_message_id, EXCLUDED.last_read_message_id),
    updated_at = now()
RETURNING last_read_message_id;

-- name: CountUnreadChatMessages :one
SELECT count(*)::bigint
FROM chat_messages AS message
WHERE message.chain_id = sqlc.arg(chain_id)
  AND message.sender_user_id = sqlc.arg(counterpart_id)
  AND message.recipient_user_id = sqlc.arg(actor_id)
  AND message.id > sqlc.arg(last_read_message_id);
