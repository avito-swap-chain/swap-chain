import { useQuery, useQueryClient } from '@tanstack/react-query'
import { isBackendConnected } from '@/shared/config/backend'
import { isServiceThread } from '../lib/thread'
import { mergeMessages } from '../lib/mergeMessages'
import type { Message, ThreadKey } from '../model/types'
import { getMessages, messageKeys } from './messagesApi'

/** Потолок ожидания по контракту: дольше держать соединение нельзя — прокси рвут его первыми. */
const WAIT_SECONDS = 25

/** Пауза между запросами: long-poll возвращается сам, опрашивать чаще нечего. */
const GAP_MS = 500

/**
 * Лента реплик, которая сама догоняет новые. Real-time у чата сделан long-poll'ом, а не SSE,
 * поэтому обычный `refetchInterval` здесь не подходит: запрос висит на сервере до появления
 * сообщения, и пауза нужна только между висениями.
 *
 * Уже показанное берём из кэша и доливаем к нему хвост — так лента не перезагружается
 * целиком на каждое новое сообщение, а курсор всегда стоит на последней известной реплике
 * (`nextAfterId` из ответа означает ровно её).
 */
export function useThreadMessages(key: ThreadKey, enabled = true) {
  const queryClient = useQueryClient()
  const queryKey = messageKeys.thread(key)

  return useQuery({
    queryKey,
    enabled,
    queryFn: async ({ signal }) => {
      const known = queryClient.getQueryData<Message[]>(queryKey) ?? []

      const fresh = await getMessages(key, {
        afterId: known.at(-1)?.id,
        // Первое чтение — сразу, чтобы экран не ждал пустого таймаута на пустой переписке.
        waitSeconds: known.length > 0 ? WAIT_SECONDS : 0,
        signal,
      })

      // Кэш перечитываем после ожидания: пока запрос висел, человек мог отправить своё
      // сообщение, и слияние со старым снимком стёрло бы его с экрана до следующего опроса.
      return mergeMessages(queryClient.getQueryData<Message[]>(queryKey) ?? known, fresh)
    },
    // Ошибка гасит цикл: долбить бэкенд запросом, который только что не прошёл, дважды
    // в секунду нельзя. На моках поддержке обновляться нечему — там она живёт на фронте.
    refetchInterval: (query) =>
      query.state.error || (isServiceThread(key) && !isBackendConnected) ? false : GAP_MS,
  })
}
