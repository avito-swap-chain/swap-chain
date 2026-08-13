import { notify } from '@/shared/model/notifications'
import { currentPersonaId } from '@/shared/model/persona'
import { sameThread, threadPath } from '../lib/thread'
import type {
  Message,
  MessageDraft,
  MessageReportReason,
  Thread,
  ThreadKey,
  ThreadRef,
} from '../model/types'
import { resetServiceThread } from './serviceThread'

/**
 * Переписки на моках: то, что делал бы бэкенд, если бы он был подключён. Лежат отдельно
 * от `messagesApi`, чтобы в нём остались только реальные вызовы.
 *
 * Разложены по персонам — переключение персоны в демо меняет и точку зрения на разговор.
 * Идентификаторы реплик — растущие числа, как у бэкенда: на этом держатся и курсор чтения,
 * и водяной знак прочитанного.
 */
interface MockThread {
  ref: ThreadRef
  messages: Message[]
  /** Ключ идемпотентности → отправленная по нему реплика. */
  sent: Map<string, Message>
  /** Водяной знак: id последней прочитанной реплики. */
  lastReadId: number
}

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

export async function listThreads(): Promise<Thread[]> {
  await delay(250)

  return (
    threadsOf(currentPersonaId())
      .map(toThread)
      // По id последней реплики, а не по времени: в моках две отправки попадают в одну
      // миллисекунду, и сортировка по дате разошлась бы случайно.
      .sort((a, b) => Number(b.lastMessage?.id) - Number(a.lastMessage?.id))
  )
}

export async function listMessages(key: ThreadKey, afterId?: string): Promise<Message[]> {
  await delay(200)
  const after = Number(afterId ?? 0)

  return (findMock(key)?.messages ?? []).filter((msg) => Number(msg.id) > after)
}

export async function send(ref: ThreadRef, draft: MessageDraft): Promise<Message> {
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

export function markRead(key: ThreadKey, lastMessageId: string): void {
  const thread = findMock(key)
  if (!thread) return

  thread.lastReadId = Math.max(thread.lastReadId, Number(lastMessageId))
}

/**
 * Жалобы на моках живут в памяти вкладки: очереди модерации без бэкенда нет, и разбирать
 * их некому. Хранятся всё же не в пустоту — по ним видно, что жалоба на одну реплику
 * повторно не заводится, как и у бэкенда.
 */
const REPORTED = new Set<string>()

export async function report(
  messageId: string,
  reason: MessageReportReason,
  comment: string,
): Promise<void> {
  await delay(400)
  REPORTED.add(`${messageId}:${reason}:${comment}`)
}

/** Сброс переписок — для тестов. */
export const resetThreads = (): void => {
  mockThreads = {}
  counter = 0
  REPORTED.clear()
  resetServiceThread()
}
