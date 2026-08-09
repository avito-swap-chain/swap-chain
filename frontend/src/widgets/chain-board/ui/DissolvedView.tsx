import { useNavigate } from 'react-router-dom'
import { findDecliner } from '@/entities/chain'
import { Banner, Button, IconCheck, IconClose } from '@/shared/ui'
import type { ChainViewProps } from '../model/types'
import { BoardFooter } from './BoardFooter'

/** DISSOLVED: кто-то отказался — обмен не стартовал, вещи разблокированы. */
export function DissolvedView({ chain, me }: ChainViewProps) {
  const navigate = useNavigate()
  const decliner = findDecliner(chain)

  // Сам себя пользователь в распавшуюся цепочку приводит только кнопкой «Выйти» — отказ от
  // предложения её не распускает. Значит и назвать это надо тем же словом, что было на кнопке.
  const reason = decliner?.isMe
    ? 'Вы вышли из цепочки.'
    : decliner
      ? `Участник ${decliner.name} отклонил обмен.`
      : 'Один из участников отказался.'

  return (
    <>
      <Banner tone="stop" icon={<IconClose size={20} />}>
        <b className="font-bold">Цепочка распалась.</b> {reason}
      </Banner>

      <Banner tone="ok" icon={<IconCheck size={20} />}>
        Вещь «{me.givesItem.title}» осталась у вас и участвует в подборе.
      </Banner>

      <BoardFooter>
        <Button fullWidth onClick={() => navigate('/exchange')}>
          К другим вариантам
        </Button>
      </BoardFooter>
    </>
  )
}
