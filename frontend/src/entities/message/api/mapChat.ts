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
 * Вещь, о которой идёт разговор. Правило от бэкенда: сначала `receiveItem` — это вещь,
 * которая придёт от собеседника ко мне, и спрашивают почти всегда о ней. Если этого ребра
 * в цепочке нет, остаётся моя вещь: тогда о состоянии спрашивает собеседник, а тема разговора
 * та же. У цепочки из двоих заполнены оба направления, и выбор всё равно однозначен.
 */
const topicOf = (thread: ChatThread) => thread.receiveItem ?? thread.giveItem

export const mapThread = (thread: ChatThread, meId: number): Thread => {
  const topic = topicOf(thread)

  return {
    chainId: String(thread.chainId),
    counterpartId: String(thread.counterpart.id),
    peerName: thread.counterpart.username,
    itemTitle: topic?.title,
    itemPhotoUrl: topic?.imageUrl ?? undefined,
    lastMessage: thread.lastMessage ? mapMessage(thread.lastMessage, meId) : undefined,
    unreadCount: thread.unreadCount,
  }
}
