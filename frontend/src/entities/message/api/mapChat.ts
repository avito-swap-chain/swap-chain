import type { ChatMessage, ChatThread } from '@/shared/api/generated/model'
import type { Message, Thread } from '../model/types'

/**
 * Реплика из контракта в нашу. Роль автора проставляется здесь, а не приходит с сервера:
 * «моё или его» — вопрос точки зрения, ровно как `isMe` у участника цепочки.
 */
export const mapMessage = (message: ChatMessage, meId: number): Message => ({
  id: String(message.id),
  author: message.sender.id === meId ? 'me' : 'them',
  text: message.text,
  createdAt: message.createdAt,
})

/**
 * Тред из контракта в наш. Тема разговора — вещь: бэкенд сам решает, какую показать
 * (полученную от собеседника, а если писали о моей — то мою), и склеивает по ней историю
 * из всех цепочек. Дедуплицировать на фронте нечего — так прямо сказано в контракте.
 */
export const mapThread = (thread: ChatThread, meId: number): Thread => ({
  itemId: String(thread.item.id),
  counterpartId: String(thread.counterpart.id),
  peerName: thread.counterpart.username,
  itemTitle: thread.item.title,
  itemPhotoUrl: thread.item.imageUrl ?? undefined,
  lastMessage: thread.lastMessage ? mapMessage(thread.lastMessage, meId) : undefined,
  unreadCount: thread.unreadCount,
})
