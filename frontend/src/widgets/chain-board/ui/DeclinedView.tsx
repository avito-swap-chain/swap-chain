import { useNavigate } from 'react-router-dom'
import { ParticipantStatusList } from '@/entities/chain'
import { Banner, Button, IconClose } from '@/shared/ui'
import type { ChainViewProps } from '../model/types'
import { BoardFooter } from './BoardFooter'

/**
 * FORMED + я DECLINED. Отказ распускает цепочку целиком, поэтому в норме это состояние
 * не наступает: сервис не ищет замену вышедшему, а собирает новый вариант с нуля — при
 * длине в три участника это дешевле, и часть тех же людей в него обычно попадает снова.
 * Вид остаётся страховкой: показывать отказавшемуся предложение с кнопкой «подтвердить»
 * нельзя ни при каких данных.
 */
export function DeclinedView({ chain, neighbours }: ChainViewProps) {
  const navigate = useNavigate()

  return (
    <>
      <Banner tone="stop" icon={<IconClose size={20} />}>
        <b className="font-bold">Вы отказались от этого варианта.</b> Вернуться в него нельзя.
      </Banner>

      <p className="text-[12.5px] text-ink-3">
        Этот вариант распался. Ваша вещь свободна и участвует в подборе — новый вариант может
        собраться и с теми же участниками.
      </p>

      <ParticipantStatusList participants={chain.participants} neighbours={neighbours} />

      <BoardFooter>
        <Button fullWidth onClick={() => navigate('/exchange')}>
          К другим вариантам
        </Button>
      </BoardFooter>
    </>
  )
}
