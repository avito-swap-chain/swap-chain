-- name: InsertMessageReport :one
INSERT INTO message_reports (reporter_user_id, message_id, reason, comment)
VALUES (
    sqlc.arg(reporter_id),
    sqlc.arg(message_id),
    sqlc.arg(reason)::report_reason,
    sqlc.narg(comment)
)
ON CONFLICT (reporter_user_id, message_id) DO NOTHING
RETURNING id;

-- name: GetModerationReport :one
SELECT report.id,
       report.reporter_user_id,
       reporter.username AS reporter_username,
       report.message_id,
       report.reason::text AS reason,
       report.comment,
       report.status::text AS status,
       report.assignee_user_id,
       assignee.username AS assignee_username,
       report.decision_comment,
       report.created_at,
       report.updated_at
FROM message_reports AS report
JOIN users AS reporter ON reporter.id = report.reporter_user_id
LEFT JOIN users AS assignee ON assignee.id = report.assignee_user_id
WHERE report.id = sqlc.arg(report_id);

-- name: GetMessageReportByReporterMessage :one
SELECT report.id,
       report.reporter_user_id,
       reporter.username AS reporter_username,
       report.message_id,
       report.reason::text AS reason,
       report.comment,
       report.status::text AS status,
       report.assignee_user_id,
       assignee.username AS assignee_username,
       report.decision_comment,
       report.created_at,
       report.updated_at
FROM message_reports AS report
JOIN users AS reporter ON reporter.id = report.reporter_user_id
LEFT JOIN users AS assignee ON assignee.id = report.assignee_user_id
WHERE report.reporter_user_id = sqlc.arg(reporter_id)
  AND report.message_id = sqlc.arg(message_id);

-- name: ListMessageReports :many
SELECT report.id,
       report.reporter_user_id,
       reporter.username AS reporter_username,
       report.message_id,
       report.reason::text AS reason,
       report.comment,
       report.status::text AS status,
       report.assignee_user_id,
       assignee.username AS assignee_username,
       report.decision_comment,
       report.created_at,
       report.updated_at
FROM message_reports AS report
JOIN users AS reporter ON reporter.id = report.reporter_user_id
LEFT JOIN users AS assignee ON assignee.id = report.assignee_user_id
WHERE report.id > sqlc.arg(after_id)
  AND (sqlc.arg(status)::text = '' OR report.status::text = sqlc.arg(status)::text)
  AND (sqlc.arg(reason)::text = '' OR report.reason::text = sqlc.arg(reason)::text)
  AND (NOT sqlc.arg(unassigned)::boolean OR report.assignee_user_id IS NULL)
  AND (sqlc.arg(assignee_id)::bigint = 0 OR report.assignee_user_id = sqlc.arg(assignee_id)::bigint)
ORDER BY report.id
LIMIT sqlc.arg(page_size);

-- name: LockMessageReport :one
SELECT report.id,
       report.status::text AS status,
       report.assignee_user_id
FROM message_reports AS report
WHERE report.id = sqlc.arg(report_id)
FOR UPDATE;

-- name: AssignMessageReport :exec
UPDATE message_reports
SET assignee_user_id = sqlc.arg(assignee_id)::bigint,
    updated_at = now()
WHERE id = sqlc.arg(report_id);

-- name: DecideMessageReport :execrows
UPDATE message_reports
SET status = sqlc.arg(status)::report_status,
    decision_comment = sqlc.arg(decision_comment)::text,
    updated_at = now()
WHERE id = sqlc.arg(report_id)
  AND status = 'open'
  AND assignee_user_id = sqlc.arg(assignee_id)::bigint;

-- name: ListThreadMessages :many
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
      (message.sender_user_id = sqlc.arg(first_user_id) AND message.recipient_user_id = sqlc.arg(second_user_id))
      OR
      (message.sender_user_id = sqlc.arg(second_user_id) AND message.recipient_user_id = sqlc.arg(first_user_id))
  )
ORDER BY message.id ASC;
