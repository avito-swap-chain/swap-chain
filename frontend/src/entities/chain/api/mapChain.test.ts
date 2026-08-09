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

  it('три состояния контракта разворачиваются в наши', () => {
    expect(mapChain(apiChain('PENDING'), 1).status).toBe('formed')
    expect(mapChain(apiChain('ACCEPTED'), 1).status).toBe('active')
    expect(mapChain(apiChain('REJECTED'), 1).status).toBe('dissolved')
  })

  it('статусы участников переводятся в решения по варианту', () => {
    const chain = mapChain(apiChain('PENDING'), 1)

    expect(chain.participants.map((p) => p.status)).toEqual(['pending', 'confirmed'])
  })

  it('отметки получения в контракте нет — признак снят у всех', () => {
    const chain = mapChain(apiChain('ACCEPTED'), 1)

    expect(chain.participants.every((p) => p.receiptConfirmed === false)).toBe(true)
  })

  it('идентификаторы приводятся к строкам: у бэкенда они числовые', () => {
    const chain = mapChain(apiChain('PENDING'), 1)

    expect(chain.id).toBe('42')
    expect(chain.participants[0].givesItem.id).toBe('1')
  })
})
