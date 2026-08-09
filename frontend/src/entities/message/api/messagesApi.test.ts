import { beforeEach, describe, expect, it } from 'vitest'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import {
  countUnread,
  getThread,
  getThreads,
  markThreadRead,
  resetThreads,
  sendMessage,
} from './messagesApi'

const [DASHA, MARK] = PERSONAS

const BIKE = { itemId: '1', itemTitle: 'Горный велосипед', peerName: 'Лена' }
const MONITOR = { itemId: '3', itemTitle: 'Монитор LG 27" IPS', peerName: 'Марк' }

/** Реплики без служебного канала: он есть всегда и к разговорам отношения не имеет. */
const dialogues = async () => (await getThreads()).filter((thread) => thread.itemId !== 'service')

describe('messagesApi — переписка о состоянии вещи', () => {
  beforeEach(() => {
    resetThreads()
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('до первого вопроса переписки по вещи нет', async () => {
    expect(await getThread(BIKE.itemId)).toBeUndefined()
    expect(await dialogues()).toEqual([])
  })

  it('мессенджер не бывает пустым: канал сервиса стоит в списке всегда', async () => {
    const threads = await getThreads()

    expect(threads).toHaveLength(1)
    expect(threads[0]).toMatchObject({ itemId: 'service', peerName: 'Авито Обмен' })
    expect(threads[0].messages[0].author).toBe('system')
  })

  it('на отправленный вопрос владелец отвечает — в ленте обе реплики подряд', async () => {
    const thread = await sendMessage({ ...BIKE, text: 'В каком состоянии вещь?' })

    expect(thread.messages).toHaveLength(2)
    expect(thread.messages[0]).toMatchObject({ author: 'me', text: 'В каком состоянии вещь?' })
    expect(thread.messages[1].author).toBe('them')
  })

  it('ответ разбирает смысл вопроса, а не повторяет его', async () => {
    const aboutKit = await sendMessage({ ...BIKE, text: 'Комплект полный?' })
    expect(aboutKit.messages[1].text).toMatch(/комплект/i)

    resetThreads()
    const aboutWear = await sendMessage({ ...BIKE, text: 'Есть царапины?' })
    expect(aboutWear.messages[1].text).toMatch(/царапина|следов/i)
  })

  it('на вопрос не о вещи отвечает нейтрально, а не выдумывает подробности', async () => {
    const thread = await sendMessage({ ...BIKE, text: 'Добрый день!' })

    expect(thread.messages[1].text).toMatch(/отвечу в течение дня/i)
  })

  it('переписка сохраняется и продолжается следующим вопросом', async () => {
    await sendMessage({ ...BIKE, text: 'В каком состоянии вещь?' })
    const thread = await sendMessage({ ...BIKE, text: 'Комплект полный?' })

    expect(thread.messages).toHaveLength(4)
    expect((await getThread(BIKE.itemId))?.messages).toHaveLength(4)
  })

  it('первое сообщение заводит тред с подписью собеседника и вещи', async () => {
    await sendMessage({ ...BIKE, itemPhotoUrl: '/mock/items/bike.jpg', text: 'Комплект полный?' })

    const [thread] = await dialogues()
    expect(thread).toMatchObject({
      itemId: BIKE.itemId,
      itemTitle: BIKE.itemTitle,
      peerName: BIKE.peerName,
      itemPhotoUrl: '/mock/items/bike.jpg',
    })
  })

  it('в списке свежий разговор идёт выше давнего', async () => {
    await sendMessage({ ...BIKE, text: 'Комплект полный?' })
    await sendMessage({ ...MONITOR, text: 'Есть битые пиксели?' })

    expect((await dialogues()).map((thread) => thread.itemId)).toEqual([
      MONITOR.itemId,
      BIKE.itemId,
    ])
  })

  it('разговоры о разных вещах не смешиваются', async () => {
    await sendMessage({ ...BIKE, text: 'Комплект полный?' })
    await sendMessage({ ...MONITOR, text: 'Есть битые пиксели?' })

    expect((await getThread(BIKE.itemId))?.messages).toHaveLength(2)
    expect((await getThread(MONITOR.itemId))?.messages).toHaveLength(2)
  })

  it('ответ собеседника делает переписку непрочитанной, а открытие — гасит счётчик', async () => {
    expect(await countUnread()).toBe(1) // канал сервиса ждёт прочтения

    await sendMessage({ ...BIKE, text: 'Комплект полный?' })
    expect(await countUnread()).toBe(2)

    await markThreadRead(BIKE.itemId)
    expect(await countUnread()).toBe(1)
    expect((await getThread(BIKE.itemId))?.unread).toBe(false)
  })

  it('прочитанная переписка снова становится непрочитанной после нового ответа', async () => {
    await sendMessage({ ...BIKE, text: 'Комплект полный?' })
    await markThreadRead(BIKE.itemId)

    const thread = await sendMessage({ ...BIKE, text: 'А царапины есть?' })
    expect(thread.unread).toBe(true)
  })

  it('счётчик считает персонально: чужие непрочитанные не видны', async () => {
    await sendMessage({ ...BIKE, text: 'Комплект полный?' })
    expect(await countUnread()).toBe(2)

    usePersonaStore.setState({ personaId: MARK.id })
    expect(await countUnread()).toBe(1) // только собственный канал сервиса
  })

  it('переписка принадлежит персоне: у другого участника её нет', async () => {
    await sendMessage({ ...BIKE, text: 'В каком состоянии вещь?' })

    usePersonaStore.setState({ personaId: MARK.id })
    expect(await getThread(BIKE.itemId)).toBeUndefined()
    expect(await dialogues()).toEqual([])

    usePersonaStore.setState({ personaId: DASHA.id })
    expect((await getThread(BIKE.itemId))?.messages).toHaveLength(2)
  })
})
