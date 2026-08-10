import { describe, expect, it } from 'vitest'
import type { ChatItemSummary, ChatMessage, ChatThread } from '@/shared/api/generated/model'
import { mapMessage, mapThread } from './mapChat'

const DASHA = { id: 1, username: 'Даша' }
const LENA = { id: 3, username: 'Лена' }

const item = (id: number, title: string): ChatItemSummary => ({
  id,
  title,
  imageUrl: `/mock/${id}.jpg`,
})

const apiMessage = (sender = LENA): ChatMessage => ({
  id: 17,
  chainId: 42,
  sender,
  recipient: sender === LENA ? DASHA : LENA,
  clientMessageId: 'c0ffee',
  text: 'Комплект полный?',
  createdAt: '2026-08-10T10:00:00Z',
})

const apiThread = (edges: Partial<ChatThread>): ChatThread => ({
  chainId: 42,
  counterpart: LENA,
  giveItem: null,
  receiveItem: null,
  lastMessage: null,
  hasUnread: false,
  unreadCount: 0,
  ...edges,
})

describe('mapMessage — реплика из контракта', () => {
  it('«моё или его» считается по владельцу сессии, а не приходит с сервера', () => {
    expect(mapMessage(apiMessage(DASHA), DASHA.id).author).toBe('me')
    expect(mapMessage(apiMessage(LENA), DASHA.id).author).toBe('them')
  })

  it('одна и та же реплика двум участникам показывается с разных сторон', () => {
    const message = apiMessage(LENA)

    expect(mapMessage(message, DASHA.id).author).toBe('them')
    expect(mapMessage(message, LENA.id).author).toBe('me')
  })
})

describe('mapThread — вещь, о которой разговор', () => {
  it('показываем ту, что придёт от собеседника ко мне', () => {
    const thread = mapThread(
      apiThread({ receiveItem: item(2, 'Наушники'), giveItem: item(1, 'Велосипед') }),
      DASHA.id,
    )

    expect(thread.itemTitle).toBe('Наушники')
    expect(thread.itemPhotoUrl).toBe('/mock/2.jpg')
  })

  it('без входящего ребра остаётся моя вещь — тема разговора та же', () => {
    const thread = mapThread(apiThread({ giveItem: item(1, 'Велосипед') }), DASHA.id)

    expect(thread.itemTitle).toBe('Велосипед')
  })

  it('переписка без вещей не пропадает, просто остаётся без темы', () => {
    expect(mapThread(apiThread({}), DASHA.id).itemTitle).toBeUndefined()
  })

  it('фото может не быть — в модели это отсутствие, а не null', () => {
    const thread = mapThread(
      apiThread({ receiveItem: { id: 2, title: 'Наушники', imageUrl: null } }),
      DASHA.id,
    )

    expect(thread.itemPhotoUrl).toBeUndefined()
  })

  it('адрес переписки — цепочка и собеседник', () => {
    const thread = mapThread(apiThread({ lastMessage: apiMessage(), unreadCount: 2 }), DASHA.id)

    expect(thread).toMatchObject({ chainId: '42', counterpartId: '3', peerName: 'Лена' })
    expect(thread.lastMessage).toMatchObject({ author: 'them', text: 'Комплект полный?' })
    expect(thread.unreadCount).toBe(2)
  })
})
