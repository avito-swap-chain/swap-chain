import { useNavigate } from 'react-router-dom'
import { cancelReason, ExchangeSummary } from '@/entities/chain'
import { Banner, Button, IconCheck, IconClose } from '@/shared/ui'
import type { ChainViewProps } from '../model/types'
import { BoardFooter } from './BoardFooter'

/**
 * CANCELLED: вещь из этого варианта успела уйти в другую цепочку — собрать его уже нельзя.
 * Отдельный вид, а не распад: никто не отказывался, просто обмен состоялся в другом месте.
 */
export function CancelledView({ chain, me, neighbours }: ChainViewProps) {
  const navigate = useNavigate()

  // Ушла моя вещь или чужая — от этого зависит, что делать дальше. Своя уже заблокирована
  // в собравшейся цепочке, и «участвует в подборе дальше» было бы про неё неправдой.
  const mineIsTaken = me.givesItem.id === chain.cancelledItemId

  return (
    <>
      {/* Не красный: человек ничего не потерял и ничего не сделал не так — обмен просто
          состоялся в другом месте. Красный здесь читался бы как сбой сервиса. */}
      <Banner tone="info" icon={<IconClose size={20} />}>
        <b className="font-bold">Вариант отменился.</b> {cancelReason(chain)}.
      </Banner>

      <ExchangeSummary me={me} neighbours={neighbours} />

      {/* Про свою вещь всё сказано баннером выше — второй раз повторять незачем. */}
      {!mineIsTaken && (
        <Banner tone="ok" icon={<IconCheck size={20} />}>
          Вещь «{me.givesItem.title}» участвует в подборе дальше.
        </Banner>
      )}

      <BoardFooter>
        <Button fullWidth onClick={() => navigate('/exchange')}>
          {mineIsTaken ? 'К моим обменам' : 'К другим вариантам'}
        </Button>
      </BoardFooter>
    </>
  )
}
