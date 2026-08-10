-- name: IsAdminUser :one
SELECT app_user.role = 'ADMIN'::user_role AS is_admin
FROM users AS app_user
WHERE app_user.id = $1
FOR SHARE;

-- name: ListAdminDeliveries :many
SELECT delivery.id,
       delivery.chain_id,
       delivery.item_id,
       item.offer_title AS item_title,
       sender.id AS sender_id,
       sender.username AS sender_username,
       recipient.id AS recipient_id,
       recipient.username AS recipient_username,
       delivery.delivery_status::text AS delivery_status,
       delivery.delivery_updated_at
FROM chain_items AS delivery
JOIN chains AS chain ON chain.id = delivery.chain_id
JOIN items AS item ON item.id = delivery.item_id
JOIN users AS sender ON sender.id = delivery.user_id
JOIN chain_items AS recipient_leg
  ON recipient_leg.chain_id = delivery.chain_id
 AND recipient_leg.next_item_id = delivery.item_id
JOIN users AS recipient ON recipient.id = recipient_leg.user_id
WHERE chain.status IN ('ACCEPTED', 'COMPLETED')
  AND item.status = 'LOCKED'
  AND delivery.id > sqlc.arg(after_id)
  AND (sqlc.arg(delivery_status)::text = ''
       OR delivery.delivery_status::text = sqlc.arg(delivery_status)::text)
ORDER BY delivery.id
LIMIT sqlc.arg(page_size);

-- name: FindAdminDeliveryChain :one
SELECT delivery.chain_id
FROM chain_items AS delivery
WHERE delivery.id = $1;

-- name: LockAdminChain :one
SELECT chain.status::text AS chain_status
FROM chains AS chain
WHERE chain.id = $1
FOR UPDATE;

-- name: LockAdminDelivery :one
SELECT delivery.delivery_status::text AS delivery_status,
       item.status::text AS item_status
FROM chain_items AS delivery
JOIN items AS item ON item.id = delivery.item_id
WHERE delivery.id = $1
FOR UPDATE OF delivery, item;

-- name: LockRecipientDelivery :one
SELECT delivery.id,
       delivery.delivery_status::text AS delivery_status,
       item.status::text AS item_status
FROM chain_items AS recipient_leg
JOIN chain_items AS delivery
  ON delivery.chain_id = recipient_leg.chain_id
 AND delivery.item_id = recipient_leg.next_item_id
JOIN items AS item ON item.id = delivery.item_id
WHERE recipient_leg.chain_id = sqlc.arg(chain_id)
  AND recipient_leg.user_id = sqlc.arg(recipient_id)
FOR UPDATE OF delivery, item;

-- name: UpdateAdminDeliveryStatus :exec
UPDATE chain_items
SET delivery_status = sqlc.arg(delivery_status)::delivery_status,
    delivery_updated_at = now()
WHERE id = sqlc.arg(delivery_id);

-- name: CreateAdminDeliveryEvent :exec
INSERT INTO admin_delivery_events (
    chain_item_id,
    actor_user_id,
    from_status,
    to_status
)
VALUES (
    sqlc.arg(delivery_id),
    sqlc.arg(actor_id),
    sqlc.arg(from_status)::delivery_status,
    sqlc.arg(to_status)::delivery_status
);

-- name: CountUnreceivedAdminDeliveries :one
SELECT count(*)
FROM chain_items AS delivery
WHERE delivery.chain_id = $1
  AND delivery.delivery_status <> 'RECEIVED';

-- name: CompleteAdminChain :execrows
UPDATE chains
SET status = 'COMPLETED',
    updated_at = now()
WHERE id = $1
  AND status = 'ACCEPTED';

-- name: GetAdminDelivery :one
SELECT delivery.id,
       delivery.chain_id,
       delivery.item_id,
       item.offer_title AS item_title,
       sender.id AS sender_id,
       sender.username AS sender_username,
       recipient.id AS recipient_id,
       recipient.username AS recipient_username,
       delivery.delivery_status::text AS delivery_status,
       delivery.delivery_updated_at
FROM chain_items AS delivery
JOIN items AS item ON item.id = delivery.item_id
JOIN users AS sender ON sender.id = delivery.user_id
JOIN chain_items AS recipient_leg
  ON recipient_leg.chain_id = delivery.chain_id
 AND recipient_leg.next_item_id = delivery.item_id
JOIN users AS recipient ON recipient.id = recipient_leg.user_id
WHERE delivery.id = $1;
