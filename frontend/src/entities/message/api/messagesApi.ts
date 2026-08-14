import { unwrap } from '@/shared/api/fetcher'
import {
  createReport,
  listChatMessages,
  listChatThreads,
  markChatThreadRead,
  sendChatMessage,
} from '@/shared/api/generated/endpoints'
import type {
  ChatMessage as ApiChatMessage,
  ChatMessageList,
  ChatThreadList,
  MessageReport,
} from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import { currentUserId } from '@/shared/model/session'
import { isServiceThread } from '../lib/thread'
import type {
  Message,
  MessageDraft,
  MessageReportReason,
  ReadOptions,
  Thread,
  ThreadKey,
  ThreadList,
  ThreadRef,
} from '../model/types'
import { mapMessage, mapThread } from './mapChat'
import * as mock from './messageMocks'
import { markServiceRead, serviceMessages, serviceThread } from './serviceThread'

/**
 * Ключи кэша TanStack Query. Список и лента реплик — соседи, а не вложены друг в друга:
 * лента висит на long-poll, и инвалидация списка после отправки обрывала бы ожидание
 * ради данных, которые тем же ожиданием и придут.
 */
export const messageKeys = {
  all: ['messages'] as const,
  list: () => [...messageKeys.all, 'threads'] as const,
  thread: ({ itemId, counterpartId }: ThreadKey) =>
    [...messageKeys.all, itemId, counterpartId] as const,
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
    threads = await mock.listThreads()
    totalUnread = threads.reduce((sum, thread) => sum + thread.unreadCount, 0)
  }

  // Служебный канал стоит первым и считается наравне с остальными: для человека это такая же
  // непрочитанная переписка, хотя на бэкенде её нет.
  const service = serviceThread()
  return {
    threads: [service, ...threads],
    totalUnread: totalUnread + service.unreadCount,
  }
}

/**
 * Реплики переписки. `afterId` — курсор: приходит только то, чего мы ещё не видели.
 * `waitSeconds` больше нуля превращает запрос в long-poll — бэкенд держит его, пока не
 * появится новое сообщение, и это весь наш real-time: отдельного канала для чата нет.
 */
export async function getMessages(key: ThreadKey, options: ReadOptions = {}): Promise<Message[]> {
  if (isServiceThread(key)) return serviceMessages()
  if (!isBackendConnected) return mock.listMessages(key, options.afterId)

  const meId = await currentUserId()
  const { messages } = unwrap<ChatMessageList>(
    await listChatMessages(
      Number(key.itemId),
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

/**
 * Отправка сообщения. Ключ идемпотентности выдаётся на действие человека, а не на запрос:
 * повтор после обрыва вернёт исходную реплику, а не заведёт вторую.
 *
 * Подписи переписки передаются вместе с текстом ради моков — там первое сообщение и заводит
 * тред. Бэкенду они не нужны: у него разговор существует с момента, как цепочка подобралась.
 */
export async function sendMessage(ref: ThreadRef, draft: MessageDraft): Promise<Message> {
  if (!isBackendConnected) return mock.send(ref, draft)

  const meId = await currentUserId()
  const sent = unwrap<ApiChatMessage>(
    await sendChatMessage(Number(ref.itemId), Number(ref.counterpartId), {
      clientMessageId: draft.clientMessageId,
      text: draft.text,
    }),
  )
  return mapMessage(sent, meId)
}

/**
 * Переписку прочитали до указанной реплики. Водяной знак назад не двигается: иначе возврат
 * к старому сообщению «разучивал» бы то, что человек уже видел.
 */
export async function markThreadRead(key: ThreadKey, lastMessageId: string): Promise<void> {
  if (isServiceThread(key)) return markServiceRead()
  if (!isBackendConnected) return mock.markRead(key, lastMessageId)

  unwrap(
    await markChatThreadRead(Number(key.itemId), Number(key.counterpartId), {
      lastReadMessageId: Number(lastMessageId),
    }),
  )
}

/**
 * Пожаловаться на реплику собеседника. Жалоба всегда на конкретное сообщение, а не на
 * человека: разбирающий её видит, что именно сказано, и не гадает по описанию.
 *
 * Повтор той же жалобы бэкенд возвращает как ту же самую — нажать второй раз безопасно.
 * На моках жалоба остаётся в памяти вкладки: разбирать её всё равно некому, а демо обязано
 * открываться без сервера.
 */
export async function reportMessage(
  messageId: string,
  reason: MessageReportReason,
  comment: string,
): Promise<void> {
  const text = comment.trim()
  // Для «другого» причина не сказана ничем, кроме текста, — без него жалоба пустая,
  // и бэкенд отвергает её как `VALIDATION_ERROR`.
  if (reason === 'other' && text === '') throw new Error('Расскажите, что случилось')

  if (!isBackendConnected) return mock.report(messageId, reason, text)

  unwrap<MessageReport>(
    await createReport({
      messageId: Number(messageId),
      reason,
      ...(text ? { comment: text } : {}),
    }),
  )
}
