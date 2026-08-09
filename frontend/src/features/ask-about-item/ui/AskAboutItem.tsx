import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { getThread, MessageList, messageKeys, type ThreadRef } from '@/entities/message'
import { MessageComposer } from '@/features/send-message'
import { Link } from 'react-router-dom'

/**
 * Разговор о вещи прямо у предложения. Согласие даётся по конкретному экземпляру, а не по
 * категории из желания, поэтому спросить о состоянии нужно там же, где принимают решение:
 * уход в мессенджер теряет контекст варианта.
 *
 * Переписка та же самая, что в разделе «Сообщения», — начатая здесь, она видна и там.
 */
export function AskAboutItem({ thread }: { thread: ThreadRef }) {
  const [open, setOpen] = useState(false)

  const { data } = useQuery({
    queryKey: messageKeys.thread(thread.itemId),
    queryFn: () => getThread(thread.itemId),
    enabled: open,
  })

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="rounded-sm text-center text-[13px] font-semibold text-brand outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand"
      >
        Спросить о состоянии
      </button>
    )
  }

  const messages = data?.messages ?? []

  return (
    <section
      aria-label={`Переписка о вещи «${thread.itemTitle}»`}
      className="flex flex-col gap-2.5"
    >
      <header className="flex items-baseline justify-between gap-2 border-b border-line pb-2">
        <p className="text-[15px] leading-5 font-bold">
          {thread.peerName}
          <span className="ml-1.5 text-[13px] font-normal text-ink-2">{thread.itemTitle}</span>
        </p>
        <button
          type="button"
          onClick={() => setOpen(false)}
          className="rounded-sm text-[13px] font-semibold text-ink-2 outline-offset-4 hover:text-ink focus-visible:outline-2 focus-visible:outline-brand"
        >
          Свернуть
        </button>
      </header>

      {messages.length > 0 && (
        <MessageList messages={messages} className="max-h-64 overflow-y-auto" />
      )}

      <MessageComposer thread={thread} empty={messages.length === 0} />

      {/* Длинный разговор дочитывают целиком — для этого есть отдельный экран. */}
      {messages.length > 0 && (
        <Link
          to={`/messages/${thread.itemId}`}
          className="rounded-sm text-center text-[13px] font-semibold text-brand outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand"
        >
          Открыть переписку
        </Link>
      )}
    </section>
  )
}
