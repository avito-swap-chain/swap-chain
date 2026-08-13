-- name: BlockUser :exec
INSERT INTO user_blocks (blocker_user_id, blocked_user_id)
VALUES (sqlc.arg(blocker_id), sqlc.arg(blocked_id))
ON CONFLICT (blocker_user_id, blocked_user_id) DO NOTHING;

-- name: UnblockUser :execrows
DELETE FROM user_blocks
WHERE blocker_user_id = sqlc.arg(blocker_id)
  AND blocked_user_id = sqlc.arg(blocked_id);

-- name: GetUserBlock :one
SELECT user_blocks.id AS block_id,
       blocked.id AS blocked_user_id,
       blocked.username AS blocked_username,
       user_blocks.created_at
FROM user_blocks
JOIN users AS blocked ON blocked.id = user_blocks.blocked_user_id
WHERE user_blocks.blocker_user_id = sqlc.arg(blocker_id)
  AND user_blocks.blocked_user_id = sqlc.arg(blocked_id);

-- name: UserExists :one
SELECT EXISTS (
    SELECT 1 FROM users WHERE id = sqlc.arg(user_id)
) AS exists;

-- name: ListUserBlocks :many
SELECT user_blocks.id AS block_id,
       blocked.id AS blocked_user_id,
       blocked.username AS blocked_username,
       user_blocks.created_at
FROM user_blocks
JOIN users AS blocked ON blocked.id = user_blocks.blocked_user_id
WHERE user_blocks.blocker_user_id = sqlc.arg(blocker_id)
  AND user_blocks.id > sqlc.arg(after_id)
ORDER BY user_blocks.id
LIMIT sqlc.arg(page_size);
