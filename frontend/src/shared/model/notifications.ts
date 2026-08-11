import { currentPersonaId } from './persona'

/**
 * Про что уведомление. Поводы — ровно те, ради которых человек возвращается в сервис:
 * ему написали, цепочка сменила состояние, вариант перестал быть возможным, вещь поехала.
 * Набор совпадает с тем, что шлёт бэкенд (`NotificationKind` в контракте).
 */
export type NotificationKind = 'message' | 'chain' | 'offer' | 'delivery'

export interface AppNotification {
  id: string
  kind: NotificationKind
  /** Заголовок строкой — как в списке уведомлений Авито. */
  title: string
  text: string
  /** Куда ведёт: уведомление без адреса бесполезно, поэтому поле обязательное. */
  to: string
  createdAt: string
  read: boolean
}

/** Список вместе со счётчиком: с бэкенда они приходят одним ответом, здесь — тоже. */
export interface NotificationList {
  items: AppNotification[]
  totalUnread: number
}

/**
 * Журнал уведомлений на моках. С подключённым бэкендом уведомления хранит он
 * (`entities/notification`), а это — то же самое для демо, которое обязано открываться
 * по ссылке без сервера.
 *
 * Живёт в `shared`, а не в своей сущности, потому что писать в него должны и цепочки,
 * и переписка: entity не может зависеть от соседней entity, а от shared — может.
 * Записи раскладываются по персонам: уведомление получает тот, кто был «я» в момент события.
 */
let byPersona: Record<string, AppNotification[]> = {}

let counter = 0

/** Записать событие текущей персоне. */
export function notify(event: Omit<AppNotification, 'id' | 'createdAt' | 'read'>): void {
  const personaId = currentPersonaId()
  const entry: AppNotification = {
    ...event,
    id: `n${++counter}`,
    createdAt: new Date().toISOString(),
    read: false,
  }
  byPersona = { ...byPersona, [personaId]: [...(byPersona[personaId] ?? []), entry] }
}

/**
 * Уведомления текущей персоны, свежие сверху. Порядок берём от записи, а не от времени:
 * два события одной миллисекунды (цепочка собралась → конкурент отменился) сортировкой
 * по дате разошлись бы случайно.
 */
export function listNotifications(): AppNotification[] {
  return [...(byPersona[currentPersonaId()] ?? [])].reverse()
}

export function countUnreadNotifications(): number {
  return (byPersona[currentPersonaId()] ?? []).filter((n) => !n.read).length
}

/** То же, что отдаёт ручка бэкенда: список и счётчик одним ответом. */
export const listForPersona = (): NotificationList => ({
  items: listNotifications(),
  totalUnread: countUnreadNotifications(),
})

/** Список открыли — всё в нём считается просмотренным, как в уведомлениях Авито. */
export function markNotificationsRead(): void {
  const personaId = currentPersonaId()
  const list = byPersona[personaId]
  if (!list?.some((n) => !n.read)) return

  byPersona = { ...byPersona, [personaId]: list.map((n) => ({ ...n, read: true })) }
}

export const markAllRead = markNotificationsRead

/** Сброс журнала — для тестов. */
export const resetNotifications = (): void => {
  byPersona = {}
  counter = 0
}
