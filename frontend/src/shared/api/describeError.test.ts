import { describe, expect, it } from 'vitest'
import { describeError } from './describeError'
import { ApiError } from './fetcher'

describe('describeError', () => {
  it.each([
    [401, 'Сессия истекла — войдите заново'],
    [403, 'Это действие сейчас недоступно'],
    [404, 'Не найдено — возможно, данные уже изменились'],
    [409, 'Уже неактуально — обновите страницу'],
    [500, 'Не удалось. Попробуйте ещё раз'],
  ])('%s → %s', (status, expected) => {
    expect(describeError(new ApiError(status, 'internal detail'))).toBe(expected)
  })

  it('текст бэкенда не показываем: он написан для разработчика', () => {
    const raw = "request body has an error: doesn't match schema #/components/schemas/LoginRequest"

    expect(describeError(new ApiError(400, raw))).not.toContain('schema')
  })

  it('409 объясняется по месту: у каждого действия свой конфликт', () => {
    const conflict = 'Вещь уже участвует в собравшейся цепочке — снять её нельзя'

    expect(describeError(new ApiError(409, ''), conflict)).toBe(conflict)
    // Для остальных кодов подсказка про конфликт не подходит и не подставляется.
    expect(describeError(new ApiError(500, ''), conflict)).not.toBe(conflict)
  })

  it('до сервера не достучались — дело в связи, а не в действиях человека', () => {
    expect(describeError(new TypeError('Failed to fetch'))).toBe(
      'Нет связи с сервером — проверьте интернет',
    )
  })

  it('свои проверки уже написаны словами для человека', () => {
    expect(describeError(new Error('Не указано, что хочется взамен'))).toBe(
      'Не указано, что хочется взамен',
    )
  })

  it('что угодно другое не оставляет человека без объяснения', () => {
    expect(describeError(undefined)).toBe('Не удалось. Попробуйте ещё раз')
  })
})
