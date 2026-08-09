/**
 * Транспорт для сгенерированного клиента. Один на все запросы: здесь база, куки сессии
 * и разбор тела — сгенерированному коду об этом знать не нужно.
 *
 * Базовый адрес берётся из `VITE_API_URL`. Если он не задан, приложение работает
 * на мок-данных и сюда не заходит — так демо остаётся запускаемым без бэкенда.
 */
const BASE_URL = import.meta.env.VITE_API_URL ?? ''

/** Ошибка запроса с кодом — по нему экраны отличают «не найдено» от «нет сессии». */
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/**
 * Ответ в форме, которую ожидает сгенерированный клиент: статус рядом с телом, без исключений.
 * Ошибочные коды описаны в контракте наравне с успешными, и разбирать их — дело вызывающего
 * (см. `unwrap`), иначе тип ответа перестанет совпадать со схемой.
 */
export async function apiFetch<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${BASE_URL}${url}`, {
    ...init,
    // Сессия демо-бэкенда живёт в куке, поэтому запросы идут с ней.
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })

  // 204 у контракта есть (logout), и тела там нет — json() на нём падает.
  const data =
    response.status === 204 ? undefined : await response.json().catch(() => response.statusText)

  return { status: response.status, data, headers: response.headers } as T
}

/**
 * Достаёт тело успешного ответа, а ошибочный код превращает в исключение: наши api-функции
 * возвращают данные, а не «данные или ошибку», — на этом держится обработка в TanStack Query.
 */
export function unwrap<T>(response: { status: number; data: unknown }): T {
  if (response.status >= 400) {
    const body = response.data as { message?: string } | string | undefined
    const message =
      (typeof body === 'object' && body?.message) ||
      (typeof body === 'string' ? body : '') ||
      `Запрос завершился с кодом ${response.status}`

    throw new ApiError(response.status, message)
  }

  return response.data as T
}
