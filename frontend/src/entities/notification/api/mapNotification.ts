import type {
  AppNotification as ApiNotification,
  NotificationKind as ApiKind,
} from '@/shared/api/generated/model'
import type { AppNotification, NotificationKind } from '@/shared/model/notifications'

const KIND: Record<ApiKind, NotificationKind> = {
  CHAIN: 'chain',
  OFFER: 'offer',
  MESSAGE: 'message',
  DELIVERY: 'delivery',
}

/**
 * Куда ведёт уведомление. Бэкенд отдаёт `chainId` и `itemId`, а не готовую ссылку —
 * маршруты дело фронта, и это правильно: адрес экрана он менять не должен.
 *
 * Переписка — исключение: тред адресуется парой «цепочка + собеседник», а собеседника
 * в уведомлении нет, поэтому ведём в общий список, а не в конкретный разговор.
 * Доставку тоже ведём в обмен: рабочее место ПВЗ — не для того, кто ждёт свою вещь.
 */
function routeOf(notification: ApiNotification): string {
  if (notification.kind === 'MESSAGE') return '/messages'
  if (notification.chainId != null) return `/exchange/${notification.chainId}`

  return '/exchange'
}

export const mapNotification = (notification: ApiNotification): AppNotification => ({
  id: String(notification.id),
  kind: KIND[notification.kind],
  title: notification.title,
  text: notification.text,
  to: routeOf(notification),
  createdAt: notification.createdAt,
  read: notification.read,
})
