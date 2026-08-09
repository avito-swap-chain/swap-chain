import { useNavigate } from 'react-router-dom'
import { findMe, findNeighbours, type Chain } from '@/entities/chain'
import { Button, Notice } from '@/shared/ui'
import { selectView } from '../lib/selectView'
import type { ChainViewProps } from '../model/types'
import { BoardFooter } from './BoardFooter'
import { OfferView } from './OfferView'
import { WaitingView } from './WaitingView'
import { HandoffView } from './HandoffView'
import { CompletedView } from './CompletedView'
import { DissolvedView } from './DissolvedView'
import { DeclinedView } from './DeclinedView'
import { CancelledView } from './CancelledView'

/** Доска цепочки: по состоянию сделки выбирает, что именно показать пользователю. */
export function ChainBoard({ chain }: { chain: Chain }) {
  const me = findMe(chain)
  const neighbours = findNeighbours(chain)

  return (
    <div className="flex flex-1 flex-col gap-3 sm:gap-3.5">
      {me && neighbours ? renderView({ chain, me, neighbours }) : <NotMyChain />}
    </div>
  )
}

/** Чужая цепочка: показывать нечего, но экран без выхода — тупик, поэтому уводим в свои обмены. */
function NotMyChain() {
  const navigate = useNavigate()

  return (
    <>
      <Notice>Вы не участвуете в этой цепочке.</Notice>
      <BoardFooter>
        <Button fullWidth onClick={() => navigate('/exchange')}>
          К моим обменам
        </Button>
      </BoardFooter>
    </>
  )
}

function renderView(props: ChainViewProps) {
  switch (selectView(props.chain)) {
    case 'offer':
      return <OfferView {...props} />
    case 'waiting':
      return <WaitingView {...props} />
    case 'handoff':
      return <HandoffView {...props} />
    case 'completed':
      return <CompletedView {...props} />
    case 'dissolved':
      return <DissolvedView {...props} />
    case 'declined':
      return <DeclinedView {...props} />
    case 'cancelled':
      return <CancelledView {...props} />
  }
}
