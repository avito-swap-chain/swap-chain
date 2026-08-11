import { unwrap } from '@/shared/api/fetcher'
import { listNotifications, markNotificationsRead } from '@/shared/api/generated/endpoints'
import type { NotificationList as ApiNotificationList } from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import * as journal from '@/shared/model/notifications'
import type { NotificationList } from '@/shared/model/notifications'
import { mapNotification } from './mapNotification'

/**
 * Ключи кэша. Список и счётчик — один ключ: бэкенд отдаёт их одним ответом, и разводить
 * их по разным запросам значит рисовать в шапке одно, а в разделе другое.
 */
export const notificationKeys = {
  all: ['notifications'] as const,
  list: () => notificationKeys.all,
}

/** Сколько показываем за раз. Дальше — «показать ещё», но пока столько и не набирается. */
const PAGE_SIZE = 50

/** Журнал текущего пользователя, свежие сверху, вместе со счётчиком для шапки. */
export async function getNotifications(): Promise<NotificationList> {
  if (!isBackendConnected) return journal.listForPersona()

  const list = unwrap<ApiNotificationList>(await listNotifications({ limit: PAGE_SIZE }))

  return {
    items: list.notifications.map(mapNotification),
    totalUnread: list.totalUnread,
  }
}

/**
 * Список открыли — всё в нём считается просмотренным, как в уведомлениях Авито.
 * Тело запроса не передаём: без `ids` бэкенд помечает прочитанным весь журнал.
 */
export async function readNotifications(): Promise<void> {
  if (!isBackendConnected) return journal.markAllRead()

  unwrap(await markNotificationsRead({}))
}
