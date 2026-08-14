import { unwrap } from '@/shared/api/fetcher'
import {
  getAdminSupportThread,
  joinSupportThread,
  leaveSupportThread,
  listAdminSupportMessages,
  listAdminSupportThreads,
  sendAdminSupportMessage,
} from '@/shared/api/generated/endpoints'
import type {
  SupportMessage,
  SupportMessageList,
  SupportThread,
} from '@/shared/api/generated/model'

/**
 * Очередь поддержки со стороны модератора. Вопрос от человека приходит в его личный тред,
 * модератор подключается к нему и отвечает — до подключения писать нельзя, и это правильно:
 * иначе в одном разговоре оказались бы сразу несколько сотрудников, не видя друг друга.
 *
 * Мок-режима нет по той же причине, что у доставок ПВЗ и жалоб: роль `ADMIN` берётся
 * из сессии, а на моках «я» это выбранная персона.
 */
export const supportKeys = {
  all: ['support-queue'] as const,
  threads: () => [...supportKeys.all, 'threads'] as const,
  thread: (id: number) => [...supportKeys.all, 'thread', id] as const,
  messages: (id: number) => [...supportKeys.all, 'messages', id] as const,
}

export async function getSupportQueue(): Promise<SupportThread[]> {
  const { threads } = unwrap<{ threads: SupportThread[] }>(await listAdminSupportThreads())
  return threads
}

export async function getSupportQueueThread(threadId: number): Promise<SupportThread> {
  return unwrap<SupportThread>(await getAdminSupportThread(threadId))
}

/** Реплики треда. Без long-poll: за стойкой открыт список, а не одно окно разговора. */
export async function getSupportQueueMessages(threadId: number): Promise<SupportMessage[]> {
  const { messages } = unwrap<SupportMessageList>(await listAdminSupportMessages(threadId, {}))
  return messages
}

export async function joinSupport(threadId: number): Promise<void> {
  unwrap(await joinSupportThread(threadId))
}

export async function leaveSupport(threadId: number): Promise<void> {
  unwrap(await leaveSupportThread(threadId))
}

export async function replySupport(threadId: number, text: string): Promise<void> {
  // Ключ идемпотентности выдаётся на отправку: повтор после обрыва не заведёт вторую реплику.
  unwrap<SupportMessage>(
    await sendAdminSupportMessage(threadId, { clientMessageId: crypto.randomUUID(), text }),
  )
}
