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
  itemId: 2,
  sender,
  recipient: sender === LENA ? DASHA : LENA,
  clientMessageId: 'c0ffee',
  text: 'Комплект полный?',
  createdAt: '2026-08-10T10:00:00Z',
})

const apiThread = (patch: Partial<ChatThread> = {}): ChatThread => ({
  item: item(2, 'Наушники'),
  counterpart: LENA,
  lastMessage: null,
  hasUnread: false,
  unreadCount: 0,
  ...patch,
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
  it('тема разговора — вещь треда: её выбирает бэкенд, фронт не гадает', () => {
    const thread = mapThread(apiThread(), DASHA.id)

    expect(thread.itemTitle).toBe('Наушники')
    expect(thread.itemPhotoUrl).toBe('/mock/2.jpg')
  })

  it('фото может не быть — в модели это отсутствие, а не null', () => {
    const thread = mapThread(
      apiThread({ item: { id: 2, title: 'Наушники', imageUrl: null } }),
      DASHA.id,
    )

    expect(thread.itemPhotoUrl).toBeUndefined()
  })

  /** С 0.10.0 адресом стала вещь: одна и та же вещь у того же человека — один разговор,
   *  сколько бы вариантов обмена с ней ни собралось. */
  it('адрес переписки — вещь и собеседник', () => {
    const thread = mapThread(apiThread({ lastMessage: apiMessage(), unreadCount: 2 }), DASHA.id)

    expect(thread).toMatchObject({ itemId: '2', counterpartId: '3', peerName: 'Лена' })
    expect(thread.lastMessage).toMatchObject({ author: 'them', text: 'Комплект полный?' })
    expect(thread.unreadCount).toBe(2)
  })
})

describe('mapMessage — предупреждения антифрода', () => {
  it('метка риска доезжает до модели: её показывают рядом с репликой', () => {
    const risky = { ...apiMessage(), riskGroup: 'VERIFICATION_CODE' as const }

    expect(mapMessage(risky, DASHA.id).risk).toBe('VERIFICATION_CODE')
  })

  /** Отсутствие метки — «бэкенд ничего не заподозрил», а не «проверено и безопасно». */
  it('обычная реплика приходит без метки', () => {
    expect(mapMessage(apiMessage(), DASHA.id).risk).toBeUndefined()
  })
})
