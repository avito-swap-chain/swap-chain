import { describe, expect, it } from 'vitest'
import type { Chain as ApiChain } from '@/shared/api/generated/model'
import { mapChain } from './mapChain'

const item = (id: number, title: string) => ({
  id,
  userId: id,
  offerTitle: title,
  offerDescription: '',
  wantDescription: '',
  imageUrls: [`/mock/${id}.jpg`],
  status: 'MATCHING' as const,
  createdAt: '2026-08-09T10:00:00Z',
  updatedAt: '2026-08-09T10:00:00Z',
})

const apiChain = (status: ApiChain['status']): ApiChain => ({
  id: 42,
  status,
  createdAt: '2026-08-09T10:00:00Z',
  expiresAt: '2026-08-10T10:00:00Z',
  participants: [
    {
      user: { id: 1, username: 'Даша' },
      giveItem: item(1, 'Горный велосипед'),
      receiveItem: item(2, 'Наушники'),
      status: 'WAITING',
    },
    {
      user: { id: 2, username: 'Марк' },
      giveItem: item(2, 'Наушники'),
      receiveItem: item(1, 'Горный велосипед'),
      status: 'APPROVED',
    },
  ],
})

describe('mapChain — цепочка из контракта в нашу модель', () => {
  it('«я» определяется по идентификатору сессии, а не приходит с сервера', () => {
    const chain = mapChain(apiChain('PENDING'), 1)

    expect(chain.participants[0]).toMatchObject({ name: 'Даша', isMe: true })
    expect(chain.participants[1].isMe).toBeUndefined()
  })

  it('состояния контракта разворачиваются в наши', () => {
    expect(mapChain(apiChain('PENDING'), 1).status).toBe('formed')
    expect(mapChain(apiChain('ACCEPTED'), 1).status).toBe('active')
    expect(mapChain(apiChain('COMPLETED'), 1).status).toBe('completed')
    expect(mapChain(apiChain('REJECTED'), 1).status).toBe('dissolved')
  })

  it('закрытая цепочка — получение отметили все: иначе COMPLETED не наступил бы', () => {
    expect(mapChain(apiChain('COMPLETED'), 1).participants.every((p) => p.receiptConfirmed)).toBe(
      true,
    )
  })

  it('статусы участников переводятся в решения по варианту', () => {
    const chain = mapChain(apiChain('PENDING'), 1)

    expect(chain.participants.map((p) => p.status)).toEqual(['pending', 'confirmed'])
  })

  it('идёт передача — отметки получения ещё нет ни у кого', () => {
    const chain = mapChain(apiChain('ACCEPTED'), 1)

    expect(chain.participants.every((p) => p.receiptConfirmed === false)).toBe(true)
  })

  it('идентификаторы приводятся к строкам: у бэкенда они числовые', () => {
    const chain = mapChain(apiChain('PENDING'), 1)

    expect(chain.id).toBe('42')
    expect(chain.participants[0].givesItem.id).toBe('1')
  })
})

// Данные со стенда, цепочка 38: Алиса отдаёт Книгу Вере, Вера Клавиатуру Борису,
// Борис Лампу Алисе. Бэкенд перечисляет участников в обратную сторону — по `giveItem`
// соседа не найти, и без `receiveItem` экран называл контрагентов зеркально.
const stand: ApiChain = {
  id: 38,
  status: 'ACCEPTED',
  createdAt: '2026-08-09T17:03:55Z',
  expiresAt: '2026-08-10T17:03:55Z',
  participants: [
    {
      user: { id: 15, username: 'Алиса' },
      giveItem: item(55, 'Книга'),
      receiveItem: item(56, 'Лампа'),
      status: 'APPROVED',
    },
    {
      user: { id: 16, username: 'Борис' },
      giveItem: item(56, 'Лампа'),
      receiveItem: item(57, 'Клавиатура'),
      status: 'APPROVED',
    },
    {
      user: { id: 17, username: 'Вера' },
      giveItem: item(57, 'Клавиатура'),
      receiveItem: item(55, 'Книга'),
      status: 'APPROVED',
    },
  ],
}

describe('mapChain — порядок участников', () => {
  it('участники разворачиваются в порядок обхода круга: каждый отдаёт следующему', () => {
    const { participants } = mapChain(stand, 15)

    expect(participants.map((p) => p.name)).toEqual(['Алиса', 'Вера', 'Борис'])
  })

  it('вещь участника получает следующий по кругу', () => {
    const { participants } = mapChain(stand, 15)
    const received = stand.participants.map((p) => [p.user.username, p.receiveItem.offerTitle])

    participants.forEach((participant, index) => {
      const next = participants[(index + 1) % participants.length]
      expect(received).toContainEqual([next.name, participant.givesItem.title])
    })
  })

  it('противоречивые данные не теряют участников', () => {
    const broken: ApiChain = {
      ...stand,
      participants: stand.participants.map((p) => ({ ...p, receiveItem: item(999, 'Ничьё') })),
    }

    expect(mapChain(broken, 15).participants).toHaveLength(3)
  })
})
