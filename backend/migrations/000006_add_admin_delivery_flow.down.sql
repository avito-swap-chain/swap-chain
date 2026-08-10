DROP INDEX IF EXISTS admin_delivery_events_chain_item_id_idx;
DROP TABLE IF EXISTS admin_delivery_events;

DROP INDEX IF EXISTS chain_items_delivery_queue_idx;
ALTER TABLE chain_items
    DROP COLUMN IF EXISTS delivery_updated_at,
    DROP COLUMN IF EXISTS delivery_status;

-- Удаляется только неиспользованная демонстрационная запись. Если от её имени
-- созданы данные обмена, пользователь сохраняется без административной роли.
DELETE FROM users AS demo_admin
WHERE demo_admin.phone = '+79009999999'
  AND demo_admin.username = 'ПВЗ Администратор (demo)'
  AND demo_admin.role = 'ADMIN'
  AND NOT EXISTS (
      SELECT 1
      FROM items
      WHERE items.user_id = demo_admin.id
  );

ALTER TABLE users
    DROP COLUMN IF EXISTS role;

DROP TYPE IF EXISTS delivery_status;
DROP TYPE IF EXISTS user_role;
