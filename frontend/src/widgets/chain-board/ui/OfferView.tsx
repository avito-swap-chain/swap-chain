import { ChainRibbon, ExchangeSummary, ParticipantStatusList, timeLeft } from '@/entities/chain'
import { RespondToExchange } from '@/features/respond-to-exchange'
import type { ChainViewProps } from '../model/types'
import { BoardFooter } from './BoardFooter'

/** FORMED + я PENDING: один из подобранных вариантов, от меня ждут ответа. */
export function OfferView({ chain, me, neighbours }: ChainViewProps) {
  const left = timeLeft(chain)

  return (
    <>
      <ExchangeSummary me={me} neighbours={neighbours} />

      {/* Лента — схема круга, список — кто уже ответил. Вещи участников называет только
          список: раньше они стояли в обоих блоках подряд, и экран повторял сам себя. */}
      <ChainRibbon participants={chain.participants} neighbours={neighbours} />

      <ParticipantStatusList participants={chain.participants} neighbours={neighbours} />

      <BoardFooter>
        <p className="text-center text-[12.5px] text-ink-2">
          Обмен начнётся, когда вариант подойдёт всем. Вещь пока остаётся у вас.
          {/* Срок ответа — рядом с кнопками, а не в шапке: решение принимают здесь,
              и знать про него надо в тот момент, когда ответ откладывают. */}
          {left && <> {left[0].toUpperCase() + left.slice(1)}.</>}
        </p>
        <RespondToExchange chainId={chain.id} />
      </BoardFooter>
    </>
  )
}
