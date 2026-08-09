import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getThread, markThreadRead, MessageList, messageKeys } from '@/entities/message'
import { MessageComposer } from '@/features/send-message'
import { IconBox, Notice, Screen, ScreenHeader } from '@/shared/ui'

/**
 * Экран переписки. Шапка держит контекст разговора — с кем и о какой вещи, — как в
 * мессенджере Авито: без неё через день не вспомнить, к какому из предложений он относится.
 */
export function ThreadPage() {
  const { itemId = '' } = useParams()
  const navigate = useNavigate()

  const { data: thread, isPending } = useQuery({
    queryKey: messageKeys.thread(itemId),
    queryFn: () => getThread(itemId),
  })

  // Открытая переписка считается прочитанной — счётчик в шапке гаснет.
  const queryClient = useQueryClient()
  useEffect(() => {
    if (!thread?.unread) return
    markThreadRead(itemId).then(() => {
      queryClient.invalidateQueries({ queryKey: messageKeys.all })
    })
  }, [itemId, thread?.unread, queryClient])

  return (
    <Screen>
      <ScreenHeader title={thread?.peerName ?? 'Переписка'} onBack={() => navigate('/messages')}>
        {thread?.itemPhotoUrl ? (
          <img
            src={thread.itemPhotoUrl}
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
        {isPending && <Notice>Загрузка…</Notice>}

        {!isPending && !thread && <Notice tone="error">Переписка не найдена</Notice>}

        {thread && (
          <>
            <p className="text-[13px] leading-4 text-ink-2">{thread.itemTitle}</p>
            <MessageList messages={thread.messages} />
            {/* Служебный канал сервиса — не диалог: отвечать в него некому. */}
            {thread.itemId !== 'service' && (
              <MessageComposer
                thread={{
                  itemId: thread.itemId,
                  itemTitle: thread.itemTitle,
                  itemPhotoUrl: thread.itemPhotoUrl,
                  peerName: thread.peerName,
                  peerAvatarUrl: thread.peerAvatarUrl,
                }}
                empty={thread.messages.length === 0}
              />
            )}
          </>
        )}
      </div>
    </Screen>
  )
}
