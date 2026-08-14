/**
 * Даты интерфейса. Правило одно на весь кабинет, как у Авито: сегодняшнее время читается
 * часами, всё остальное — днём и месяцем. Год добавляется, только когда он не текущий:
 * «14 августа 2025» в списке за эту неделю выглядит опечаткой.
 *
 * Раньше эти три строки были переписаны в карточке переписки, карточке уведомления,
 * отзывах, профиле и двух экранах модерации — и разошлись между собой.
 */

const time = (date: Date) =>
  date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })

const isToday = (date: Date) => date.toDateString() === new Date().toDateString()

/** «14:03» для сегодняшних, «21 апр.» для остальных — как в списке диалогов Авито. */
export const formatWhen = (iso: string): string => {
  const date = new Date(iso)
  return isToday(date)
    ? time(date)
    : date.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
}

/** «12 августа 2026» — полная дата там, где событие давнее: отзывы, регистрация. */
export const formatDate = (iso: string): string =>
  new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })

/** «12 августа в 03:43» — журнал и очередь модерации: там важна и минута. */
export const formatDateTime = (iso: string): string =>
  new Date(iso).toLocaleString('ru-RU', {
    day: 'numeric',
    month: 'long',
    hour: '2-digit',
    minute: '2-digit',
  })
