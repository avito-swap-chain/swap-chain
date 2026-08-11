BEGIN;

DROP INDEX IF EXISTS notifications_user_unread_idx;
DROP INDEX IF EXISTS notifications_user_created_idx;
DROP TABLE IF EXISTS notifications;

COMMIT;
