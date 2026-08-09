import { useNavigate } from 'react-router-dom'
import { ParticipantStatusList } from '@/entities/chain'
import { Banner, Button, IconClose } from '@/shared/ui'
import type { ChainViewProps } from '../model/types'
import { BoardFooter } from './BoardFooter'

/** FORMED + я DECLINED: я отказался от этого варианта, но цепочка жива — ищут мне замену. */
export function DeclinedView({ chain, neighbours }: ChainViewProps) {
  const navigate = useNavigate()

  return (
    <>
      <Banner tone="stop" icon={<IconClose size={20} />}>
        <b className="font-bold">Вы отказались от этого варианта.</b> Вернуться в него нельзя.
      </Banner>

      <p className="text-[12.5px] text-ink-3">Остальным участникам сервис ищет вам замену.</p>

      <ParticipantStatusList participants={chain.participants} neighbours={neighbours} />

      <BoardFooter>
        <Button fullWidth onClick={() => navigate('/exchange')}>
          К другим вариантам
        </Button>
      </BoardFooter>
    </>
  )
}
