import type { VisionAnalysisResponse } from './generated/model'

/**
 * Подписка на поток событий бэкенда.
 *
 * Сгенерированный `subscribeEvents` для этого не годится: он построен на обычном `fetch`,
 * а `text/event-stream` не заканчивается — запрос повис бы навсегда. Нужен нативный
 * `EventSource`: он сам разбирает формат, сам переподключается и понимает именованные события,
 * а сервер шлёт именно их.
 *
 * Почти всякое событие — только сигнал «данные устарели», а не сами данные: реплея сервер
 * не хранит, поэтому и при подключении, и при каждом переподключении состояние перечитывается
 * целиком. Исключение — распознавание фото: его результат приходит именно в событии и нигде
 * больше не лежит, отдельной ручки за ним нет.
 */

/** События бэкенда. Полезная нагрузка не разбирается — кроме распознавания фото. */
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
 * Кто ждёт результат распознавания. Подписка живёт отдельно от компонента с формой:
 * поток один на приложение и открывается в `app`, а форма появляется и исчезает.
 */
type VisionListener = (result: VisionAnalysisResponse | Error) => void

const visionListeners = new Set<VisionListener>()

/** Ждать результат анализа фото. Возвращает функцию отписки. */
export function onVisionResult(listener: VisionListener): () => void {
  visionListeners.add(listener)
  return () => {
    visionListeners.delete(listener)
  }
}

const notifyVision = (result: VisionAnalysisResponse | Error) => {
  for (const listener of visionListeners) listener(result)
}

/**
 * Поток открыт и сервер прислал `stream.connected`. Контракт требует дождаться этого
 * до запуска анализа: результат придёт только в поток, а пропущенное не переотправляется.
 */
let connected = false
let awaitingConnection: (() => void)[] = []

export function whenStreamConnected(timeoutMs = 5000): Promise<boolean> {
  if (connected) return Promise.resolve(true)

  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve(false), timeoutMs)
    awaitingConnection.push(() => {
      clearTimeout(timer)
      resolve(true)
    })
  })
}

const markConnected = () => {
  connected = true
  for (const waiting of awaitingConnection) waiting()
  awaitingConnection = []
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
  source.addEventListener('stream.connected', () => {
    markConnected()
    onConnected()
  })
  for (const event of ITEM_EVENTS) source.addEventListener(event, onItems)
  for (const event of CHAIN_EVENTS) source.addEventListener(event, onChains)

  // Результат распознавания — единственное событие, из которого мы читаем данные.
  // Полезная нагрузка лежит в поле `data` конверта; кривой JSON здесь означает,
  // что анализ до формы не доедет, и человеку об этом надо сказать.
  source.addEventListener('vision.analysis.completed', (event) => {
    try {
      const { data } = JSON.parse((event as MessageEvent<string>).data)
      notifyVision(data as VisionAnalysisResponse)
    } catch {
      notifyVision(new Error('Не удалось разобрать ответ распознавания'))
    }
  })

  source.addEventListener('vision.analysis.failed', () => {
    notifyVision(new Error('Не удалось распознать вещь на фото'))
  })

  return () => {
    connected = false
    source.close()
  }
}
