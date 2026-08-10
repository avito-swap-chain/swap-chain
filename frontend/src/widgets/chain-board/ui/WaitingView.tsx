import { ExchangeSummary, ParticipantStatusList } from '@/entities/chain'
import { LeaveChain } from '@/features/leave-chain'
import { Banner, IconCheck } from '@/shared/ui'
import type { ChainViewProps } from '../model/types'
import { BoardFooter } from './BoardFooter'

/** FORMED + я CONFIRMED: вариант мне подошёл — ждём ответа остальных участников. */
export function WaitingView({ chain, me, neighbours }: ChainViewProps) {
  return (
    <>
      <Banner tone="ok" icon={<IconCheck size={20} />}>
        <b className="font-bold">Вариант вам подходит.</b> Ждём ответа остальных.
      </Banner>

      <ExchangeSummary me={me} neighbours={neighbours} />

      {/* Полосы «подошёл N из M» здесь нет: счёт стоит в шапке экрана, а кто именно ответил —
          видно построчно в списке. Полоса была третьим показом одного и того же числа. */}
      <ParticipantStatusList participants={chain.participants} neighbours={neighbours} />

      <p className="text-[12.5px] leading-relaxed text-ink-3">
        Вещь пока не заблокирована — она участвует и в других вариантах. Заблокируется, когда этот
        вариант подойдёт всем: остальные варианты с ней тогда отменятся.
      </p>

      <BoardFooter>
        <p className="text-center text-[12.5px] text-ink-2">
          Пока обмен не запущен, вы можете выйти без последствий.
        </p>
        <LeaveChain chainId={chain.id} />
      </BoardFooter>
    </>
  )
}
