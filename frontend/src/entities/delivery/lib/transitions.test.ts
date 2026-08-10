import { describe, expect, it } from 'vitest'
import type { DeliveryStatus } from '../model/types'
import { nextStatus } from './transitions'

const ALL: DeliveryStatus[] = ['AWAITING_PVZ', 'AT_PVZ', 'IN_DELIVERY', 'RECEIVED']

describe('переходы доставки', () => {
  it('ведут вещь по пути шаг за шагом', () => {
    expect(nextStatus('AWAITING_PVZ')).toBe('AT_PVZ')
    expect(nextStatus('AT_PVZ')).toBe('IN_DELIVERY')
    expect(nextStatus('IN_DELIVERY')).toBe('RECEIVED')
  })

  it('у полученной вещи следующего шага нет', () => {
    expect(nextStatus('RECEIVED')).toBe(undefined)
  })

  // Ни один шаг не ведёт назад: пропустить приёмку или отменить её кнопкой нельзя,
  // иначе интерфейс обещал бы больше, чем умеет стойка ПВЗ.
  it('в ожидание вещь не возвращается', () => {
    expect(ALL.map(nextStatus)).not.toContain('AWAITING_PVZ')
  })
})
