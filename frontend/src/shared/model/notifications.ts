import { currentPersonaId } from './persona'

/**
 * Про что уведомление. Три повода — ровно те, ради которых человек возвращается в сервис:
 * ему написали, цепочка сменила состояние, вариант перестал быть возможным.
 */
export type NotificationKind = 'message' | 'chain' | 'offer'

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

/**
 * Журнал уведомлений живёт в `shared`, а не в своей сущности, потому что писать в него
 * должны и цепочки, и переписка: entity не может зависеть от соседней entity, а от shared —
 * может. Читает журнал раздел уведомлений.
 *
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

/** Список открыли — всё в нём считается просмотренным, как в уведомлениях Авито. */
export function markNotificationsRead(): void {
  const personaId = currentPersonaId()
  const list = byPersona[personaId]
  if (!list?.some((n) => !n.read)) return

  byPersona = { ...byPersona, [personaId]: list.map((n) => ({ ...n, read: true })) }
}

/** Сброс журнала — для тестов. */
export const resetNotifications = (): void => {
  byPersona = {}
  counter = 0
}
