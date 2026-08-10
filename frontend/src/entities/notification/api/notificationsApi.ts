import {
  countUnreadNotifications,
  listNotifications,
  markNotificationsRead,
  type AppNotification,
} from '@/shared/model/notifications'

/** Ключи кэша. Счётчик под общим префиксом — инвалидируется вместе со списком. */
export const notificationKeys = {
  all: ['notifications'] as const,
  list: () => notificationKeys.all,
  unread: () => [...notificationKeys.all, 'unread'] as const,
}

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

/** Журнал текущей персоны, свежие сверху. */
export async function getNotifications(): Promise<AppNotification[]> {
  await delay(200)
  return listNotifications()
}

/** Сколько уведомлений не просмотрено — счётчик на колокольчике. */
export async function getUnreadCount(): Promise<number> {
  await delay(150)
  return countUnreadNotifications()
}

/** Список открыли — счётчик гаснет. */
export async function readNotifications(): Promise<void> {
  markNotificationsRead()
}
