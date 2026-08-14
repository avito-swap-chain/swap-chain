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
import { BlockUser } from '@/features/block-user'
import { ReportUser } from '@/features/report-user'
import { ReportMessage } from '@/features/report-message'
import { MessageComposer } from '@/features/send-message'
import { IconBox, Notice, Screen, ScreenHeader } from '@/shared/ui'
import { isBackendConnected } from '@/shared/config/backend'
import { asset } from '@/shared/lib'

/**
 * Экран переписки. Шапка держит контекст разговора — с кем и о какой вещи, — как в
 * мессенджере Авито: без неё через день не вспомнить, к какому из предложений он относится.
 *
 * Адрес диалога — вещь и собеседник: разговор идёт о конкретной вещи, и если она попала
 * сразу в несколько вариантов обмена, история остаётся одна — так решил бэкенд.
 */
export function ThreadPage() {
  const { itemId = '', counterpartId = '' } = useParams()
  const key = { itemId, counterpartId }
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
    markThreadRead({ itemId, counterpartId }, lastMessageId).then(() => {
      queryClient.invalidateQueries({ queryKey: messageKeys.list() })
    })
  }, [itemId, counterpartId, lastMessageId, unread, queryClient])

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
            {/* Пожаловаться можно только на чужую реплику: на свою бэкенд отвечает
                `SELF_REPORT`, да и незачем. */}
            <MessageList
              messages={messages}
              action={(message) =>
                message.author === 'them' ? <ReportMessage messageId={message.id} /> : null
              }
            />
            {/* В поддержку писать можно: с контракта 0.11.0 за тредом стоит настоящий
                разговор с модератором. На моках отвечать в него по-прежнему некому. */}
            {(!isServiceThread(thread) || isBackendConnected) && (
              <MessageComposer thread={thread} empty={messages.length === 0} />
            )}

            {/* Защита — там же, где разговор. Жалоба на реплику живёт у самих реплик,
                а здесь — то, что относится к человеку целиком: пожаловаться на него
                и заблокировать. Раньше вход в жалобу был только в чужом профиле, и его
                не находили: до профиля из диалога ещё надо догадаться дойти. */}
            {!isServiceThread(thread) && (
              <div className="flex flex-col gap-2 border-t border-line-2 pt-3">
                <ReportUser userId={counterpartId} />
                <BlockUser userId={counterpartId} name={thread.peerName} />
              </div>
            )}
          </>
        )}
      </div>
    </Screen>
  )
}
