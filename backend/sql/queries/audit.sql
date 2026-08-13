-- name: InsertAuditLog :exec
INSERT INTO admin_audit_log (admin_user_id, action, target_type, target_id, metadata)
VALUES (
    sqlc.arg(admin_id),
    sqlc.arg(action)::audit_action,
    sqlc.arg(target_type),
    sqlc.arg(target_id),
    sqlc.arg(metadata)
);

-- name: ListAuditLog :many
SELECT log.id,
       log.admin_user_id,
       admin.username AS admin_username,
       log.action::text AS action,
       log.target_type,
       log.target_id,
       log.metadata,
       log.created_at
FROM admin_audit_log AS log
JOIN users AS admin ON admin.id = log.admin_user_id
WHERE (sqlc.narg(before_id)::bigint IS NULL OR log.id < sqlc.narg(before_id)::bigint)
  AND (sqlc.arg(action)::text = '' OR log.action::text = sqlc.arg(action)::text)
  AND (sqlc.arg(admin_id)::bigint = 0 OR log.admin_user_id = sqlc.arg(admin_id)::bigint)
  AND (sqlc.arg(target_type)::text = '' OR log.target_type = sqlc.arg(target_type)::text)
ORDER BY log.id DESC
LIMIT sqlc.arg(page_size);
