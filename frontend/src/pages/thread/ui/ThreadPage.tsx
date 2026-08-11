import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  getThreads,
  isServiceThread,
  markThreadRead,
  MessageList,
  messageKeys,
  sameThread,
  useThreadMessages,
} from '@/entities/message'
import { MessageComposer } from '@/features/send-message'
import { IconBox, Notice, Screen, ScreenHeader } from '@/shared/ui'
import { asset } from '@/shared/lib'

/**
 * Экран переписки. Шапка держит контекст разговора — с кем и о какой вещи, — как в
 * мессенджере Авито: без неё через день не вспомнить, к какому из предложений он относится.
 *
 * Адрес диалога — цепочка и собеседник: переписка привязана к ребру круга обмена, и с одним
 * и тем же человеком в двух цепочках это два разных разговора.
 */
export function ThreadPage() {
  const { chainId = '', counterpartId = '' } = useParams()
  const key = { chainId, counterpartId }
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  // Подписи берём из списка переписок: отдельной ручки одного треда в контракте нет,
  // а список всё равно нужен шапке кабинета и обновляется сам.
  const {
    data: thread,
    isPending: threadPending,
    isError: threadFailed,
  } = useQuery({
    queryKey: messageKeys.list(),
    queryFn: getThreads,
    select: (list) => list.threads.find((item) => sameThread(item, key)),
  })

  const { data: messages = [], isPending } = useThreadMessages(key)

  const lastMessageId = messages.at(-1)?.id
  const unread = thread?.unreadCount ?? 0

  useEffect(() => {
    if (!unread || !lastMessageId) return

    // Открытая переписка считается прочитанной — счётчик в шапке гаснет. Водяной знак
    // ставим по последней показанной реплике, а не «по всему треду»: отметить непоказанное
    // мы не вправе, а список после этого надо перечитать — счётчик живёт в нём.
    markThreadRead({ chainId, counterpartId }, lastMessageId).then(() => {
      queryClient.invalidateQueries({ queryKey: messageKeys.list() })
    })
  }, [chainId, counterpartId, lastMessageId, unread, queryClient])

  return (
    <Screen>
      <ScreenHeader title={thread?.peerName ?? 'Переписка'} onBack={() => navigate('/messages')}>
        {thread?.itemPhotoUrl ? (
          <img
            src={asset(thread.itemPhotoUrl)}
            alt=""
            className="size-9 rounded-chip object-cover"
            width={36}
            height={36}
          />
        ) : (
          <span className="grid size-9 place-items-center rounded-chip bg-line-2 text-ink-3">
            <IconBox size={18} />
          </span>
        )}
      </ScreenHeader>

      <div className="flex flex-col gap-3 p-4">
        {isPending && threadPending && <Notice>Загрузка…</Notice>}

        {/* Список переписок не загрузился — это не значит, что разговора нет. */}
        {threadFailed && <Notice tone="error">Не удалось загрузить переписку</Notice>}

        {!threadPending && !threadFailed && !thread && (
          <Notice tone="error">Переписка не найдена</Notice>
        )}

        {thread && (
          <>
            {thread.itemTitle && (
              <p className="text-[13px] leading-4 text-ink-2">{thread.itemTitle}</p>
            )}
            <MessageList messages={messages} />
            {/* Служебный канал сервиса — не диалог: отвечать в него некому. */}
            {!isServiceThread(thread) && (
              <MessageComposer thread={thread} empty={messages.length === 0} />
            )}
          </>
        )}
      </div>
    </Screen>
  )
}
