/**
 * Подписка на поток событий бэкенда.
 *
 * Сгенерированный `subscribeEvents` для этого не годится: он построен на обычном `fetch`,
 * а `text/event-stream` не заканчивается — запрос повис бы навсегда. Нужен нативный
 * `EventSource`: он сам разбирает формат, сам переподключается и понимает именованные события,
 * а сервер шлёт именно их.
 *
 * Событие — только сигнал «данные устарели», а не сами данные. Реплея сервер не хранит:
 * пропущенное за время обрыва не придёт, поэтому и при подключении, и при каждом
 * переподключении надо перечитать состояние заново.
 */

/** События бэкенда. Полезная нагрузка не разбирается: она нужна только серверу. */
export type BackendEvent =
  | 'item.created'
  | 'item.status.updated'
  | 'chain.created'
  | 'chain.updated'
  | 'chain.accepted'
  | 'chain.rejected'

const ITEM_EVENTS: BackendEvent[] = ['item.created', 'item.status.updated']

const CHAIN_EVENTS: BackendEvent[] = [
  'chain.created',
  'chain.updated',
  'chain.accepted',
  'chain.rejected',
]

interface Handlers {
  /** Что-то случилось с вещами: сменился статус разбора, вещь сняли с обмена. */
  onItems: () => void
  /** Что-то случилось с цепочками: появилась, кто-то ответил, собралась, распалась. */
  onChains: () => void
  /** Соединение установлено или восстановлено — состояние надо перечитать целиком. */
  onConnected: () => void
}

/**
 * Открывает поток и возвращает функцию отписки.
 *
 * `withCredentials` обязателен: сессия живёт в куке, без неё поток ответит 401.
 * Адрес относительный — фронт и API отдаются с одного origin (см. `shared/api/fetcher`).
 */
export function subscribeToBackendEvents({ onItems, onChains, onConnected }: Handlers): () => void {
  const source = new EventSource('/api/v1/events', { withCredentials: true })

  // `stream.connected` приходит и на первое подключение, и после каждого разрыва —
  // одного обработчика хватает на оба случая.
  source.addEventListener('stream.connected', onConnected)
  for (const event of ITEM_EVENTS) source.addEventListener(event, onItems)
  for (const event of CHAIN_EVENTS) source.addEventListener(event, onChains)

  return () => source.close()
}
