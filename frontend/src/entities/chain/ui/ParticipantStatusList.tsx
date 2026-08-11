import { Link } from 'react-router-dom'
import { cx } from '@/shared/lib'
import { Status, type StatusTone } from '@/shared/ui'
import { displayName, isNeighbour, type Neighbours } from '../lib/participants'
import type { ChainParticipant, ParticipantStatus } from '../model/types'
import { ParticipantAvatar } from './ParticipantAvatar'

// Статус участника набран текстом справа, как колонка «Статус» в заказах Авито.
// Цветных значков на аватарах и подложки под своей строкой больше нет: они дублировали
// подпись и делали список пёстрым. Оранжевый остаётся единственной строке, где ход за
// пользователем, — её и надо увидеть первой.
// Подписи — про согласие с вариантом, а не про заключённую сделку: лайк означает
// «мне подходит», и обмен стартует, только когда так ответят все.
const STATUS_VIEW: Record<
  ParticipantStatus,
  { tone: StatusTone; label: string; myTone: StatusTone; myLabel: string }
> = {
  confirmed: {
    tone: 'neutral',
    label: 'Подходит',
    myTone: 'neutral',
    myLabel: 'Вам подходит',
  },
  pending: {
    tone: 'muted',
    label: 'Ожидаем',
    myTone: 'attention',
    myLabel: 'Ваш ход',
  },
  declined: {
    tone: 'stop',
    label: 'Не подошло',
    myTone: 'stop',
    myLabel: 'Вы отказались',
  },
}

interface ParticipantStatusListProps {
  participants: ChainParticipant[]
  /** Соседи по кругу: с ними пользователь встретится, поэтому их видно подробнее. */
  neighbours: Neighbours
}

/**
 * Кто уже согласился, а кого ждём — прозрачность цепочки до согласия.
 * Состав показываем целиком, но не поровну: рейтинг нужен там, где будет встреча,
 * поэтому у дальних участников остаётся только имя и вещь, приглушённые цветом.
 */
export function ParticipantStatusList({ participants, neighbours }: ParticipantStatusListProps) {
  return (
    <div className="overflow-hidden rounded-card border border-line">
      {participants.map((p) => {
        const view = STATUS_VIEW[p.status]
        const near = p.isMe || isNeighbour(p, neighbours)

        return (
          <div
            key={p.userId}
            className="flex items-center gap-2.5 border-b border-line-2 px-3 py-2 last:border-b-0"
          >
            <ParticipantAvatar
              participant={p}
              className={cx(
                'size-9 bg-line-2 text-[14px] font-bold',
                near ? 'text-ink-2' : 'text-ink-3',
              )}
            />

            <div className="min-w-0 flex-1">
              <b className={cx('block truncate text-[13.5px] font-bold', !near && 'text-ink-2')}>
                {/* На соседа по кругу можно посмотреть: вещь отдают ему, и «кто он такой»
                    — вопрос, который возникает до согласия. Себя открывать незачем. */}
                {p.isMe ? (
                  displayName(p)
                ) : (
                  <Link
                    to={`/users/${p.userId}`}
                    className="rounded-sm outline-offset-2 hover:text-brand focus-visible:outline-2 focus-visible:outline-brand"
                  >
                    {displayName(p)}
                  </Link>
                )}
                {near && p.rating != null && (
                  <span className="font-normal text-ink-3"> · {p.rating.toFixed(1)} ★</span>
                )}
              </b>
              <span
                className={cx('block truncate text-[12px]', near ? 'text-ink-2' : 'text-ink-3')}
              >
                отдаёт: {p.givesItem.title}
              </span>
            </div>

            <Status tone={p.isMe ? view.myTone : view.tone} className="shrink-0">
              {p.isMe ? view.myLabel : view.label}
            </Status>
          </div>
        )
      })}
    </div>
  )
}
