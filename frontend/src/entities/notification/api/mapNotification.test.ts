import { describe, expect, it } from 'vitest'
import type { AppNotification as ApiNotification } from '@/shared/api/generated/model'
import { mapNotification } from './mapNotification'

const notification = (patch: Partial<ApiNotification> = {}): ApiNotification => ({
  id: 1,
  kind: 'CHAIN',
  title: 'Вариант обмена отменён',
  text: 'Вариант обмена отменён',
  read: false,
  createdAt: '2026-08-14T03:00:00Z',
  ...patch,
})

describe('mapNotification — причина отмены человеческим языком', () => {
  it('служебный код заменяется фразой: «item_unavailable» человеку ничего не говорит', () => {
    const mapped = mapNotification(
      notification({ text: 'Вариант обмена отменён: item_unavailable' }),
    )

    expect(mapped.text).toBe('Вариант обмена отменён: одна из вещей ушла в другой обмен')
  })

  it('незнакомый код остаётся как есть — потерять причину хуже, чем показать её кодом', () => {
    const mapped = mapNotification(notification({ text: 'Вариант обмена отменён: quantum_flux' }))

    expect(mapped.text).toBe('Вариант обмена отменён: quantum_flux')
  })

  it('обычный текст не трогаем', () => {
    expect(mapNotification(notification({ text: 'Цепочка собралась' })).text).toBe(
      'Цепочка собралась',
    )
  })

  it('переписка ведёт в общий список: собеседника в уведомлении нет', () => {
    expect(mapNotification(notification({ kind: 'MESSAGE', itemId: 7 })).to).toBe('/messages')
  })
})
