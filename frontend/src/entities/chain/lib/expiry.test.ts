import { describe, expect, it } from 'vitest'
import { timeLeft } from './expiry'
import type { Chain } from '../model/types'

const NOW = Date.parse('2026-08-14T12:00:00Z')

const chain = (over: Partial<Chain> = {}): Chain => ({
  id: 'c1',
  status: 'formed',
  participants: [],
  expiresAt: '2026-08-15T06:00:00Z',
  ...over,
})

describe('timeLeft', () => {
  it('показывает остаток в часах', () => {
    expect(timeLeft(chain(), NOW)).toBe('ответить осталось 18 ч')
  })

  it('переходит на минуты в последний час', () => {
    expect(timeLeft(chain({ expiresAt: '2026-08-14T12:40:00Z' }), NOW)).toBe(
      'ответить осталось 40 мин',
    )
  })

  // Ноль минут читался бы как «уже поздно», хотя ответить ещё можно.
  it('в последнюю минуту всё равно предлагает ответить', () => {
    expect(timeLeft(chain({ expiresAt: '2026-08-14T12:00:20Z' }), NOW)).toBe(
      'ответить осталось 1 мин',
    )
  })

  it('говорит прямо, что срок вышел', () => {
    expect(timeLeft(chain({ expiresAt: '2026-08-14T11:59:00Z' }), NOW)).toBe(
      'время на ответ вышло',
    )
  })

  // Срок есть только у предложения: после общего согласия отсчёт уже не про ответ.
  it('молчит на цепочке не в статусе «предложение»', () => {
    expect(timeLeft(chain({ status: 'active' }), NOW)).toBeUndefined()
    expect(timeLeft(chain({ status: 'completed' }), NOW)).toBeUndefined()
  })

  // Моки живут без срока, и это не повод рисовать пустую подпись.
  it('молчит, когда срока нет', () => {
    expect(timeLeft(chain({ expiresAt: undefined }), NOW)).toBeUndefined()
    expect(timeLeft(chain({ expiresAt: 'не дата' }), NOW)).toBeUndefined()
  })
})
