import { beforeEach, describe, expect, it } from 'vitest'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import { isServiceThread, SERVICE_THREAD } from '../lib/thread'
import type { ThreadRef } from '../model/types'
import { resetThreads } from './messageMocks'
import { getMessages, getThreads, markThreadRead, sendMessage } from './messagesApi'

const [DASHA, MARK] = PERSONAS

/** Даша спрашивает Лену о велосипеде в цепочке c1, Марка о мониторе — в c5. */
const BIKE: ThreadRef = {
  chainId: 'c1',
  counterpartId: 'u3',
  peerName: 'Лена',
  itemTitle: 'Горный велосипед',
}
const MONITOR: ThreadRef = {
  chainId: 'c5',
  counterpartId: 'u2',
  peerName: 'Марк',
  itemTitle: 'Монитор LG 27" IPS',
}

let keys = 0
const ask = (ref: ThreadRef, text: string) =>
  sendMessage(ref, { text, clientMessageId: `k${++keys}` })

/** Разговоры без служебного канала: он есть всегда и к переписке участников отношения не имеет. */
const dialogues = async () => (await getThreads()).threads.filter((t) => !isServiceThread(t))

/** Прочитать всё, что сейчас показано, — ровно это делает открытый экран переписки. */
const readAll = async (ref: ThreadRef) => {
  const messages = await getMessages(ref)
  await markThreadRead(ref, messages.at(-1)?.id ?? '0')
}

describe('messagesApi — переписка о состоянии вещи', () => {
  beforeEach(() => {
    resetThreads()
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('до первого вопроса переписки нет', async () => {
    expect(await getMessages(BIKE)).toEqual([])
    expect(await dialogues()).toEqual([])
  })

  it('мессенджер не бывает пустым: канал сервиса стоит первым всегда', async () => {
    const { threads } = await getThreads()

    expect(threads).toHaveLength(1)
    expect(threads[0]).toMatchObject({ peerName: 'Авито Обмен', ...SERVICE_THREAD })
    expect(threads[0].lastMessage?.author).toBe('system')
  })

  it('на отправленный вопрос владелец отвечает — в ленте обе реплики подряд', async () => {
    await ask(BIKE, 'В каком состоянии вещь?')

    const messages = await getMessages(BIKE)
    expect(messages).toHaveLength(2)
    expect(messages[0]).toMatchObject({ author: 'me', text: 'В каком состоянии вещь?' })
    expect(messages[1].author).toBe('them')
  })

  it('ответ разбирает смысл вопроса, а не повторяет его', async () => {
    await ask(BIKE, 'Комплект полный?')
    expect((await getMessages(BIKE))[1].text).toMatch(/комплект/i)

    resetThreads()
    await ask(BIKE, 'Есть царапины?')
    expect((await getMessages(BIKE))[1].text).toMatch(/царапина|следов/i)
  })

  it('на вопрос не о вещи отвечает нейтрально, а не выдумывает подробности', async () => {
    await ask(BIKE, 'Добрый день!')

    expect((await getMessages(BIKE))[1].text).toMatch(/отвечу в течение дня/i)
  })

  it('первое сообщение заводит переписку с подписями собеседника и вещи', async () => {
    await ask({ ...BIKE, itemPhotoUrl: '/mock/items/bike.jpg' }, 'Комплект полный?')

    const [thread] = await dialogues()
    expect(thread).toMatchObject({
      chainId: BIKE.chainId,
      counterpartId: BIKE.counterpartId,
      peerName: 'Лена',
      itemTitle: 'Горный велосипед',
      itemPhotoUrl: '/mock/items/bike.jpg',
    })
    expect(thread.lastMessage?.author).toBe('them')
  })

  it('курсор отдаёт только то, чего мы ещё не видели', async () => {
    await ask(BIKE, 'Комплект полный?')
    const [mine, reply] = await getMessages(BIKE)

    expect(await getMessages(BIKE, { afterId: mine.id })).toEqual([reply])
    expect(await getMessages(BIKE, { afterId: reply.id })).toEqual([])
  })

  it('повтор с тем же ключом идемпотентности не заводит вторую реплику', async () => {
    const first = await sendMessage(BIKE, { text: 'Комплект полный?', clientMessageId: 'once' })
    const repeat = await sendMessage(BIKE, { text: 'Комплект полный?', clientMessageId: 'once' })

    expect(repeat).toEqual(first)
    expect(await getMessages(BIKE)).toHaveLength(2)
  })

  it('переписка продолжается следующим вопросом', async () => {
    await ask(BIKE, 'В каком состоянии вещь?')
    await ask(BIKE, 'Комплект полный?')

    expect(await getMessages(BIKE)).toHaveLength(4)
  })

  it('в списке свежий разговор идёт выше давнего', async () => {
    await ask(BIKE, 'Комплект полный?')
    await ask(MONITOR, 'Есть битые пиксели?')

    expect((await dialogues()).map((t) => t.chainId)).toEqual([MONITOR.chainId, BIKE.chainId])
  })

  it('разговоры в разных цепочках не смешиваются', async () => {
    await ask(BIKE, 'Комплект полный?')
    await ask(MONITOR, 'Есть битые пиксели?')

    expect(await getMessages(BIKE)).toHaveLength(2)
    expect(await getMessages(MONITOR)).toHaveLength(2)
  })

  it('один и тот же собеседник в другой цепочке — другой разговор', async () => {
    const other = { ...BIKE, chainId: 'c9', itemTitle: 'Кофеварка' }
    await ask(BIKE, 'Комплект полный?')
    await ask(other, 'А кофеварка на ходу?')

    expect(await getMessages(BIKE)).toHaveLength(2)
    expect(await getMessages(other)).toHaveLength(2)
    expect(await dialogues()).toHaveLength(2)
  })
})

describe('messagesApi — непрочитанное', () => {
  beforeEach(() => {
    resetThreads()
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('канал сервиса ждёт прочтения, пока его не открыли', async () => {
    expect((await getThreads()).totalUnread).toBe(1)

    await markThreadRead(SERVICE_THREAD, 'service')
    expect((await getThreads()).totalUnread).toBe(0)
  })

  it('ответ собеседника поднимает счётчик, открытие переписки — гасит', async () => {
    await ask(BIKE, 'Комплект полный?')
    expect((await getThreads()).totalUnread).toBe(2) // ответ Лены и канал сервиса

    await readAll(BIKE)
    expect((await getThreads()).totalUnread).toBe(1)
    expect((await dialogues())[0].unreadCount).toBe(0)
  })

  it('после нового ответа переписка снова непрочитана', async () => {
    await ask(BIKE, 'Комплект полный?')
    await readAll(BIKE)
    await ask(BIKE, 'А царапины есть?')

    expect((await dialogues())[0].unreadCount).toBe(1)
  })

  it('водяной знак не двигается назад: возврат к старой реплике ничего не разучивает', async () => {
    await ask(BIKE, 'Комплект полный?')
    const [mine] = await getMessages(BIKE)

    await readAll(BIKE)
    await markThreadRead(BIKE, mine.id)

    expect((await dialogues())[0].unreadCount).toBe(0)
  })

  it('счётчик считает персонально: чужие непрочитанные не видны', async () => {
    await ask(BIKE, 'Комплект полный?')
    expect((await getThreads()).totalUnread).toBe(2)

    usePersonaStore.setState({ personaId: MARK.id })
    expect((await getThreads()).totalUnread).toBe(1) // только собственный канал сервиса
    expect(await dialogues()).toEqual([])
    expect(await getMessages(BIKE)).toEqual([])
  })
})
