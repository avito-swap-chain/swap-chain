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
 * Переписка — исключение: тред адресуется парой «вещь + собеседник», а собеседника
 * в уведомлении нет, поэтому ведём в общий список, а не в конкретный разговор.
 * Доставку тоже ведём в обмен: рабочее место ПВЗ — не для того, кто ждёт свою вещь.
 */
function routeOf(notification: ApiNotification): string {
  if (notification.kind === 'MESSAGE') return '/messages'
  if (notification.chainId != null) return `/exchange/${notification.chainId}`

  return '/exchange'
}

/**
 * Причина отмены варианта приходит в тексте уведомления служебным кодом — бэкенд
 * подставляет его как есть («Вариант обмена отменён: item_unavailable»). Человеку такой
 * текст не говорит ничего, поэтому коды переводим здесь. Незнакомый код не трогаем: пусть
 * лучше останется как есть, чем пропадёт причина целиком.
 */
const REASON_TEXT: Record<string, string> = {
  item_unavailable: 'одна из вещей ушла в другой обмен',
  item_withdrawn: 'вещь сняли с обмена',
  item_changed: 'вещь изменили',
  declined: 'участник отказался',
  expired: 'время на ответ вышло',
  blocked: 'участники не могут меняться друг с другом',
  unknown: 'подробностей нет',
}

const humanizeReason = (text: string): string =>
  text.replace(/:\s*([a-z_]+)\s*$/, (whole, code: string) =>
    // `hasOwn`, а не просто чтение: `constructor` и прочие члены прототипа проходят
    // по маске кода, и уведомление показало бы исходник функции вместо причины.
    Object.hasOwn(REASON_TEXT, code) ? `: ${REASON_TEXT[code]}` : whole,
  )

export const mapNotification = (notification: ApiNotification): AppNotification => ({
  id: String(notification.id),
  kind: KIND[notification.kind],
  title: notification.title,
  text: humanizeReason(notification.text),
  to: routeOf(notification),
  createdAt: notification.createdAt,
  read: notification.read,
})
