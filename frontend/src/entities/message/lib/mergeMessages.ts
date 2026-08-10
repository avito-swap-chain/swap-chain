import type { Message } from '../model/types'

/**
 * Долив свежих реплик к уже показанным. Дедупликация по id обязательна: отправленное
 * сообщение попадает в ленту сразу ответом сервера, а long-poll, начатый до отправки,
 * вернёт его же — без проверки реплика задвоилась бы.
 *
 * Когда нового нет (long-poll вышел по таймауту), возвращаем прежний массив, а не копию:
 * новая ссылка на те же данные перерисовывала бы ленту вхолостую после каждого ожидания.
 */
export function mergeMessages(known: Message[], fresh: Message[]): Message[] {
  if (fresh.length === 0) return known

  const seen = new Set(known.map((message) => message.id))
  const added = fresh.filter((message) => !seen.has(message.id))

  return added.length > 0 ? [...known, ...added] : known
}
