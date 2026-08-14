import { Link } from 'react-router-dom'
import { cx, formatWhen } from '@/shared/lib'
import type { AppNotification, NotificationKind } from '@/shared/model/notifications'
import { IconBox, IconChat, IconClock, IconSwap } from '@/shared/ui'

/* Иконка по поводу: сообщение, состояние цепочки, судьба варианта, движение вещи.
 * Цвет несёт смысл — азур для переписки, зелёный для движения цепочки, оранжевый
 * для потерянного варианта, нейтральный для доставки: там от человека ничего не требуется. */
const ICONS: Record<NotificationKind, { icon: typeof IconChat; tone: string }> = {
  message: { icon: IconChat, tone: 'bg-brand-pale text-brand' },
  chain: { icon: IconSwap, tone: 'bg-ok-bg text-ok' },
  offer: { icon: IconClock, tone: 'bg-attention-bg text-attention' },
  delivery: { icon: IconBox, tone: 'bg-line-2 text-ink-2' },
}

/**
 * Строка уведомления: значок повода, заголовок, объяснение и время. Непрочитанное
 * подсвечено бледной заливкой — в списке Авито новое тоже видно, не вчитываясь.
 */
export function NotificationCard({ notification }: { notification: AppNotification }) {
  const { icon: Icon, tone } = ICONS[notification.kind]

  return (
    <Link
      to={notification.to}
      className={cx(
        'flex items-start gap-3 rounded-card px-3 py-3 outline-offset-2 hover:bg-line-2 focus-visible:outline-2 focus-visible:outline-brand',
        !notification.read && 'bg-brand-pale/60',
      )}
    >
      <span className={cx('grid size-10 shrink-0 place-items-center rounded-full', tone)}>
        <Icon size={20} />
      </span>

      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex items-baseline gap-2">
          <span className="truncate text-[15px] leading-5 font-bold">{notification.title}</span>
          <span className="ml-auto shrink-0 text-[13px] leading-4 text-ink-2">
            {formatWhen(notification.createdAt)}
          </span>
        </span>
        <span className="text-[15px] leading-5 text-ink-2">{notification.text}</span>
      </span>
    </Link>
  )
}
