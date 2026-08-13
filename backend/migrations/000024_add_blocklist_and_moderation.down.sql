BEGIN;

-- Причина 'blocked' вводится этой миграцией; перед сужением словаря
-- существующие значения 'blocked' нормализуются в 'unknown'.
UPDATE chain_rejections SET reason = 'unknown' WHERE reason = 'blocked';
ALTER TABLE chain_rejections DROP CONSTRAINT chain_rejections_reason_check;
ALTER TABLE chain_rejections ADD CONSTRAINT chain_rejections_reason_check
    CHECK (reason IN (
        'declined',
        'expired',
        'item_unavailable',
        'item_changed',
        'item_withdrawn',
        'unknown'
    ));

DROP TABLE admin_audit_log;
DROP TABLE message_reports;
DROP TABLE user_blocks;

DROP TYPE audit_action;
DROP TYPE report_reason;
DROP TYPE report_status;

COMMIT;
