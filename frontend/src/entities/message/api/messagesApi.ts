import { unwrap } from '@/shared/api/fetcher'
import {
  listChatMessages,
  listChatThreads,
  markChatThreadRead,
  sendChatMessage,
} from '@/shared/api/generated/endpoints'
import type {
  ChatMessage as ApiChatMessage,
  ChatMessageList,
  ChatThreadList,
} from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import { notify } from '@/shared/model/notifications'
import { currentPersonaId } from '@/shared/model/persona'
import { currentUserId } from '@/shared/model/session'
import { isServiceThread, sameThread, threadPath } from '../lib/thread'
import type { Message, Thread, ThreadKey, ThreadList, ThreadRef } from '../model/types'
import { mapMessage, mapThread } from './mapChat'

/**
 * Ключи кэша TanStack Query. Список и лента реплик — соседи, а не вложены друг в друга:
 * лента висит на long-poll, и инвалидация списка после отправки обрывала бы ожидание
 * ради данных, которые тем же ожиданием и придут.
 */
export const messageKeys = {
  all: ['messages'] as const,
  list: () => [...messageKeys.all, 'threads'] as const,
  thread: ({ chainId, counterpartId }: ThreadKey) =>
    [...messageKeys.all, chainId, counterpartId] as const,
}

/**
 * Заготовки вопросов о состоянии. Это не украшение: соразмерность в цепочке обеспечена тем,
 * что каждый получает названную им вещь, — но устроит ли конкретный экземпляр, видно только
 * из разговора с владельцем, и спросить надо до согласия.
 */
export const QUICK_QUESTIONS = [
  'В каком состоянии вещь?',
  'Есть дефекты или следы использования?',
  'Комплект полный?',
  'Давно пользуетесь?',
] as const

/** Что передаётся в отправку: текст и ключ идемпотентности, выданный на само действие. */
export interface MessageDraft {
  text: string
  clientMessageId: string
}

/** Параметры чтения ленты: курсор и сколько секунд бэкенду держать запрос, ожидая нового. */
export interface ReadOptions {
  afterId?: string
  waitSeconds?: number
  signal?: AbortSignal
}

/**
 * Служебный канал сервиса — он же первый в списке. У Авито мессенджер никогда не бывает
 * совсем пустым: сверху всегда стоит канал самого Авито. Здесь он объясняет правило,
 * которое иначе пришлось бы объяснять в интерфейсе цепочки.
 *
 * Канал целиком фронтовый и согласован с бэкендом именно так: это пояснение интерфейса,
 * а не переписка между участниками, — в контракте отправитель всегда пользователь.
 * Отметка о прочтении живёт в памяти вкладки: хранить её негде и незачем.
 */
const SERVICE_MESSAGE: Message = {
  id: 'service',
  author: 'system',
  text: 'В обмене нет цен и доплат: вы получаете ровно то, что указали в желании. Прежде чем согласиться, уточните у владельца состояние вещи — по фото и описанию видно не всё.',
  createdAt: new Date(Date.now() - 36e5).toISOString(),
}

let serviceRead = false

const serviceThread = (): Thread => ({
  chainId: 'service',
  counterpartId: 'service',
  peerName: 'Авито Обмен',
  itemTitle: 'Обмен без доплат',
  lastMessage: SERVICE_MESSAGE,
  unreadCount: serviceRead ? 0 : 1,
})

/**
 * Ответы владельца подобраны по смыслу вопроса — так виден сам сценарий «уточнить состояние»,
 * а не просто эхо. В бою здесь живой собеседник, поэтому логика намеренно тривиальная.
 */
const REPLIES: [RegExp, (title: string) => string][] = [
  [
    /дефект|царап|скол|потёрт|потерт|след|состоян|битый|рабоч/i,
    () =>
      'Состояние хорошее: следов почти нет, есть небольшая царапина на корпусе снизу — на работу не влияет. Могу прислать фото этого места.',
  ],
  [
    /комплект|коробк|документ|чек|зарядк|провод/i,
    () => 'Комплект полный: коробка, зарядка и документы на месте.',
  ],
  [/давно|сколько|лет|год|польз|купил/i, () => 'Пользуюсь полтора года, хранил дома, не ронял.'],
  [
    /фото|снимок|видео|покаж/i,
    (title) => `Сфотографирую ${title.toLowerCase()} при дневном свете и пришлю сегодня.`,
  ],
  [
    /встрет|обмен|когда|где|переда|привез|привёз/i,
    () => 'По передаче договоримся, когда цепочка соберётся — сервис подскажет очередь.',
  ],
]

const FALLBACK = 'Отвечу в течение дня. Если что-то важно уточнить до обмена — спрашивайте.'

const replyTo = (text: string, itemTitle = 'вещь'): string => {
  const matched = REPLIES.find(([pattern]) => pattern.test(text))
  return matched ? matched[1](itemTitle) : FALLBACK
}

/**
 * Мок вместо бэкенда. Переписки раскладываются по персонам: переключение персоны в демо
 * меняет и точку зрения на разговор. Идентификаторы реплик — растущие числа, как у бэкенда:
 * на этом держатся и курсор чтения, и водяной знак прочитанного.
 */
interface MockThread {
  ref: ThreadRef
  messages: Message[]
  /** Ключ идемпотентности → отправленная по нему реплика. */
  sent: Map<string, Message>
  /** Водяной знак: id последней прочитанной реплики. */
  lastReadId: number
}

let mockThreads: Record<string, MockThread[]> = {}

let counter = 0

const message = (author: Message['author'], text: string): Message => ({
  id: String(++counter),
  author,
  text,
  createdAt: new Date().toISOString(),
})

const threadsOf = (personaId: string): MockThread[] => (mockThreads[personaId] ??= [])

const findMock = (key: ThreadKey): MockThread | undefined =>
  threadsOf(currentPersonaId()).find((thread) => sameThread(thread.ref, key))

const openMock = (ref: ThreadRef): MockThread => {
  const existing = findMock(ref)
  if (existing) return existing

  const created: MockThread = { ref, messages: [], sent: new Map(), lastReadId: 0 }
  threadsOf(currentPersonaId()).push(created)
  return created
}

const toThread = (thread: MockThread): Thread => ({
  ...thread.ref,
  lastMessage: thread.messages.at(-1),
  unreadCount: thread.messages.filter(
    (msg) => msg.author === 'them' && Number(msg.id) > thread.lastReadId,
  ).length,
})

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

/** Все переписки текущего пользователя, свежие сверху, вместе со счётчиком для шапки. */
export async function getThreads(): Promise<ThreadList> {
  let threads: Thread[]
  let totalUnread: number

  if (isBackendConnected) {
    const meId = await currentUserId()
    const list = unwrap<ChatThreadList>(await listChatThreads())
    // Бэкенд уже отдал список в порядке последней реплики — пересортировывать нечего.
    threads = list.threads.map((thread) => mapThread(thread, meId))
    totalUnread = list.totalUnreadCount
  } else {
    await delay(250)
    threads = threadsOf(currentPersonaId())
      .map(toThread)
      // По id последней реплики, а не по времени: в моках две отправки попадают в одну
      // миллисекунду, и сортировка по дате разошлась бы случайно.
      .sort((a, b) => Number(b.lastMessage?.id) - Number(a.lastMessage?.id))
    totalUnread = threads.reduce((sum, thread) => sum + thread.unreadCount, 0)
  }

  // Служебный канал стоит первым и считается наравне с остальными: для человека это такая же
  // непрочитанная переписка, хотя на бэкенде её нет.
  return {
    threads: [serviceThread(), ...threads],
    totalUnread: totalUnread + (serviceRead ? 0 : 1),
  }
}

/**
 * Реплики переписки. `afterId` — курсор: приходит только то, чего мы ещё не видели.
 * `waitSeconds` больше нуля превращает запрос в long-poll — бэкенд держит его, пока не
 * появится новое сообщение, и это весь наш real-time: отдельного канала для чата нет.
 */
export async function getMessages(key: ThreadKey, options: ReadOptions = {}): Promise<Message[]> {
  if (isServiceThread(key)) return [SERVICE_MESSAGE]

  if (isBackendConnected) {
    const meId = await currentUserId()
    const { messages } = unwrap<ChatMessageList>(
      await listChatMessages(
        Number(key.chainId),
        Number(key.counterpartId),
        {
          afterId: options.afterId ? Number(options.afterId) : undefined,
          waitSeconds: options.waitSeconds,
        },
        { signal: options.signal },
      ),
    )
    return messages.map((msg) => mapMessage(msg, meId))
  }

  await delay(200)
  const after = Number(options.afterId ?? 0)
  return (findMock(key)?.messages ?? []).filter((msg) => Number(msg.id) > after)
}

/**
 * Отправка сообщения. Ключ идемпотентности выдаётся на действие человека, а не на запрос:
 * повтор после обрыва вернёт исходную реплику, а не заведёт вторую.
 *
 * Подписи переписки передаются вместе с текстом ради моков — там первое сообщение и заводит
 * тред. Бэкенду они не нужны: у него разговор существует с момента, как цепочка подобралась.
 */
export async function sendMessage(ref: ThreadRef, draft: MessageDraft): Promise<Message> {
  if (isBackendConnected) {
    const meId = await currentUserId()
    const sent = unwrap<ApiChatMessage>(
      await sendChatMessage(Number(ref.chainId), Number(ref.counterpartId), {
        clientMessageId: draft.clientMessageId,
        text: draft.text,
      }),
    )
    return mapMessage(sent, meId)
  }

  await delay(300)
  const thread = openMock(ref)

  const already = thread.sent.get(draft.clientMessageId)
  if (already) return already

  const mine = message('me', draft.text)
  thread.sent.set(draft.clientMessageId, mine)
  // Ответ владельца приходит сразу же: в демо важно показать сценарий целиком, а не ожидание.
  // В ленту он попадёт следующим опросом — так же, как пришёл бы с бэкенда.
  thread.messages.push(mine, message('them', replyTo(draft.text, ref.itemTitle)))

  notify({
    kind: 'message',
    title: `Новое сообщение от ${ref.peerName}`,
    text: thread.messages.at(-1)?.text ?? '',
    to: threadPath(ref),
  })

  return mine
}

/**
 * Переписку прочитали до указанной реплики. Водяной знак назад не двигается: иначе возврат
 * к старому сообщению «разучитывал» бы то, что человек уже видел.
 */
export async function markThreadRead(key: ThreadKey, lastMessageId: string): Promise<void> {
  if (isServiceThread(key)) {
    serviceRead = true
    return
  }

  if (isBackendConnected) {
    unwrap(
      await markChatThreadRead(Number(key.chainId), Number(key.counterpartId), {
        lastReadMessageId: Number(lastMessageId),
      }),
    )
    return
  }

  const thread = findMock(key)
  if (!thread) return
  thread.lastReadId = Math.max(thread.lastReadId, Number(lastMessageId))
}

/** Сброс переписок — для тестов. */
export const resetThreads = (): void => {
  mockThreads = {}
  counter = 0
  serviceRead = false
}
