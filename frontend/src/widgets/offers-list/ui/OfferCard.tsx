import { Link } from 'react-router-dom'
import {
  cancelReason,
  countConfirmed,
  ExchangeSummary,
  findMe,
  findNeighbours,
  type Chain,
} from '@/entities/chain'
import { AskAboutItem } from '@/features/ask-about-item'
import { RespondToExchange } from '@/features/respond-to-exchange'
import { cx } from '@/shared/lib'
import { Card } from '@/shared/ui'

interface OfferCardProps {
  chain: Chain
  /** Сколько живых вариантов держат мою вещь из этого предложения — 1 значит соперников нет. */
  variants: number
}

/**
 * Одно предложение в списке: что отдаю и что получаю, кому и от кого, сколько участников уже
 * согласны — и ответ прямо отсюда. На экран цепочки уходят, чтобы разобраться, а не чтобы решить.
 */
export function OfferCard({ chain, variants }: OfferCardProps) {
  const me = findMe(chain)
  const neighbours = findNeighbours(chain)
  if (!me || !neighbours) return null

  const total = chain.participants.length
  const cancelled = chain.status === 'cancelled'

  return (
    <Card className="flex flex-col gap-3 p-3.5">
      {/* Отменённое предложение приглушено, но остаётся на месте: исчезнувшая карточка
          читается как поломка, а человеку надо понять, куда делся вариант. */}
      <div className={cx('flex flex-col gap-2', cancelled && 'opacity-60')}>
        <ExchangeSummary me={me} neighbours={neighbours} />

        <p className="text-[12.5px] text-ink-2">
          Участников: {total} · согласны {countConfirmed(chain)} из {total}
          {/* Не объясняем механику заново — только помечаем, что вариант не единственный. */}
          {!cancelled && variants > 1 && ` · ещё ${variants - 1} с этой вещью`}
        </p>
      </div>

      {cancelled ? (
        <p className="text-[12.5px] text-ink-2">
          <b className="font-bold">{cancelReason(chain)}.</b>
          {/* Про свою вещь добавлять нечего: она уже в собравшейся цепочке, а не «в подборе». */}
          {me.givesItem.id !== chain.cancelledItemId && ' Вещь участвует в подборе дальше.'}
        </p>
      ) : (
        <RespondToExchange chainId={chain.id} />
      )}

      {/* Спросить о вещи можно, пока решение не принято: после отмены собеседник уже неактуален. */}
      {!cancelled && (
        <AskAboutItem
          thread={{
            itemId: neighbours.giver.givesItem.id,
            itemTitle: neighbours.giver.givesItem.title,
            itemPhotoUrl: neighbours.giver.givesItem.photoUrl,
            peerName: neighbours.giver.name,
            peerAvatarUrl: neighbours.giver.avatarUrl,
          }}
        />
      )}

      <Link
        to={`/exchange/${chain.id}`}
        className="rounded-sm text-center text-[13px] font-semibold text-brand outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand"
      >
        Разобраться в цепочке
      </Link>
    </Card>
  )
}
