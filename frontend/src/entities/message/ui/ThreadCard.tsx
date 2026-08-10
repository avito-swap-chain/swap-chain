import { Link } from 'react-router-dom'
import { asset, cx } from '@/shared/lib'
import { IconBox } from '@/shared/ui'
import { threadPath } from '../lib/thread'
import type { Thread } from '../model/types'

/** «14:03» для сегодняшних, «21 апр.» для остальных — как в списке диалогов Авито. */
const formatWhen = (iso: string): string => {
  const date = new Date(iso)
  const today = new Date()
  const sameDay = date.toDateString() === today.toDateString()
  return sameDay
    ? date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
    : date.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
}

/**
 * Строка списка переписок: фото вещи с аватаром собеседника в углу, имя, о какой вещи речь
 * и последняя реплика. Порядок ровно как у Авито — сначала «с кем», потом «о чём»:
 * вещь здесь и есть тема разговора.
 */
export function ThreadCard({ thread }: { thread: Thread }) {
  const last = thread.lastMessage

  return (
    <Link
      to={threadPath(thread)}
      className="flex items-center gap-3 rounded-card px-2 py-2.5 outline-offset-2 hover:bg-line-2 focus-visible:outline-2 focus-visible:outline-brand"
    >
      <span className="relative shrink-0">
        {thread.itemPhotoUrl ? (
          <img
            src={asset(thread.itemPhotoUrl)}
            alt=""
            className="size-14 rounded-chip object-cover"
            width={56}
            height={56}
          />
        ) : (
          <span className="grid size-14 place-items-center rounded-chip bg-line-2 text-ink-3">
            <IconBox size={24} />
          </span>
        )}
        {thread.peerAvatarUrl && (
          <img
            src={asset(thread.peerAvatarUrl)}
            alt=""
            className="absolute -bottom-1 -left-1 size-6 rounded-full border-2 border-card object-cover"
            width={24}
            height={24}
          />
        )}
      </span>

      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex items-baseline gap-2">
          <span className="truncate text-[15px] leading-5 font-bold">{thread.peerName}</span>
          {last && (
            <span className="ml-auto shrink-0 text-[13px] leading-4 text-ink-2">
              {formatWhen(last.createdAt)}
            </span>
          )}
        </span>
        {thread.itemTitle && (
          <span className="truncate text-[13px] leading-4 text-ink-2">{thread.itemTitle}</span>
        )}
        {/* Непрочитанное набрано тёмным и жирным — как в любом мессенджере. */}
        {last && (
          <span
            className={cx(
              'truncate text-[15px] leading-5',
              thread.unreadCount > 0 ? 'font-bold text-ink' : 'text-ink-2',
            )}
          >
            {last.text}
          </span>
        )}
      </span>
    </Link>
  )
}
