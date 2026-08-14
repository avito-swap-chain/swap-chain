import { unwrap } from '@/shared/api/fetcher'
import {
  getSupportThread as getSupportThreadRequest,
  listSupportMessages,
  markSupportRead,
  sendSupportMessage as sendSupportMessageRequest,
} from '@/shared/api/generated/endpoints'
import type {
  SupportMessage as ApiSupportMessage,
  SupportMessageList,
  SupportThread as ApiSupportThread,
} from '@/shared/api/generated/model'
import type { Message, MessageDraft, ReadOptions, Thread } from '../model/types'

/**
 * Переписка с поддержкой. До 0.11.0 это был фронтовый канал-пояснение: в контракте
 * отправителем сообщения всегда числился пользователь, и разговаривать с сервисом было
 * не с кем. Теперь у треда есть бэкенд, к нему подключаются модераторы, и он отвечает.
 *
 * Живёт в том же списке переписок и первой строкой — как «Поддержка Авито» у Авито:
 * человек, которому страшно отдавать вещь незнакомцу, должен видеть, что спросить есть кого.
 */

/** Кто написал. `SYSTEM` — не человек, а сам сервис: у него и пузырь другой. */
const authorOf = (message: ApiSupportMessage): Message['author'] => {
  if (message.senderType === 'USER') return 'me'
  return message.senderType === 'SYSTEM' ? 'system' : 'them'
}

const mapSupportMessage = (message: ApiSupportMessage): Message => ({
  id: String(message.id),
  author: authorOf(message),
  // Имя модератора приписываем к реплике: в поддержке отвечают разные люди, и «кто-то
  // из поддержки» звучит как автоответчик.
  text:
    message.senderType === 'MODERATOR' && message.sender
      ? `${message.sender.username}: ${message.text}`
      : message.text,
  createdAt: message.createdAt,
})

/** Тред поддержки строкой списка переписок. Адрес у него служебный — вещи здесь нет. */
export const mapSupportThread = (thread: ApiSupportThread): Thread => ({
  itemId: 'support',
  counterpartId: 'support',
  peerName: 'Поддержка Авито',
  itemTitle: thread.moderators.length > 0 ? 'На связи модератор' : 'Обмен без доплат',
  lastMessage: thread.lastMessage ? mapSupportMessage(thread.lastMessage) : undefined,
  unreadCount: thread.unreadCount,
})

export async function getSupport(): Promise<Thread> {
  return mapSupportThread(unwrap<ApiSupportThread>(await getSupportThreadRequest()))
}

/** Реплики поддержки тем же курсором и тем же long-poll, что и обычная переписка. */
export async function getSupportMessages(options: ReadOptions = {}): Promise<Message[]> {
  const { messages } = unwrap<SupportMessageList>(
    await listSupportMessages(
      {
        afterId: options.afterId ? Number(options.afterId) : undefined,
        waitSeconds: options.waitSeconds,
      },
      { signal: options.signal },
    ),
  )
  return messages.map(mapSupportMessage)
}

export async function sendToSupport(draft: MessageDraft): Promise<Message> {
  const sent = unwrap<ApiSupportMessage>(
    await sendSupportMessageRequest({
      clientMessageId: draft.clientMessageId,
      text: draft.text,
    }),
  )
  return mapSupportMessage(sent)
}

export async function readSupport(lastMessageId: string): Promise<void> {
  unwrap(await markSupportRead({ lastReadMessageId: Number(lastMessageId) }))
}
