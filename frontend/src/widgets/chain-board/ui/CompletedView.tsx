import { useNavigate } from 'react-router-dom'
import { ExchangeSummary } from '@/entities/chain'
import { LeaveReview } from '@/features/leave-review'
import { Banner, Button, IconCheck } from '@/shared/ui'
import type { ChainViewProps } from '../model/types'
import { BoardFooter } from './BoardFooter'

/** COMPLETED: все передали и подтвердили получение — цепочка закрыта. */
export function CompletedView({ chain, me, neighbours }: ChainViewProps) {
  const navigate = useNavigate()

  return (
    <>
      <Banner tone="ok" icon={<IconCheck size={20} />}>
        <b className="font-bold">Обмен завершён.</b> Все вещи нашли новых владельцев.
      </Banner>

      <ExchangeSummary me={me} neighbours={neighbours} past />

      {/* «Цепочка закрыта» уже сказано баннером и статусом в шапке — здесь только то,
          чего человек ещё не знает: куда делась полученная вещь. */}
      <p className="text-[12.5px] leading-relaxed text-ink-3">
        Полученная вещь появится в списке ваших объявлений.
      </p>

      {/* Оцениваем того, от кого получили вещь: с ним человек и имел дело — переписывался,
          ждал передачу. Об остальных участниках круга ему сказать нечего. */}
      <LeaveReview
        chainId={chain.id}
        target={{ userId: neighbours.giver.userId, name: neighbours.giver.name }}
      />

      <BoardFooter>
        <Button fullWidth onClick={() => navigate('/')}>
          К моим объявлениям
        </Button>
      </BoardFooter>
    </>
  )
}
