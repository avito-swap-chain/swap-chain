import { countReceipts, ExchangeSummary } from '@/entities/chain'
import { ConfirmReceipt } from '@/features/confirm-receipt'
import { Banner, IconCheck, type Step, Steps } from '@/shared/ui'
import type { ChainViewProps } from '../model/types'
import { BoardFooter } from './BoardFooter'
import { ChainProgress } from './ChainProgress'

/** ACTIVE: все подтвердили — остались шаги передачи вещей. */
export function HandoffView({ chain, me, neighbours }: ChainViewProps) {
  const { receiver, giver } = neighbours
  const receive = giver.givesItem
  const done = me.receiptConfirmed

  // Свою вещь пользователь передал, когда сосед отметил её получение, — это единственный
  // объективный сигнал о ходе передачи, который есть у цепочки. Дальше ход снова за ним.
  const handedOver = receiver.receiptConfirmed

  // Передача идёт через пункт выдачи, а не из рук в руки: участники не встречаются и не знают
  // адресов друг друга, а «заморозка вещи» становится физической, а не статусом в базе.
  const steps: Step[] = [
    {
      state: handedOver ? 'done' : 'current',
      content: (
        <>
          Сдайте «{me.givesItem.title}» в пункт выдачи — вещь получит{' '}
          <b className="font-bold">{receiver.name}</b>
        </>
      ),
    },
    {
      state: handedOver ? 'done' : 'todo',
      content: <>Дождитесь, пока ПВЗ передаст её в доставку</>,
    },
    {
      state: handedOver ? 'current' : 'todo',
      content: (
        <>
          Заберите «{receive.title}» в пункте выдачи — вещь отдаёт{' '}
          <b className="font-bold">{giver.name}</b>
        </>
      ),
    },
    {
      state: 'todo',
      content: <>Отметьте, что получили вещь</>,
    },
  ]

  return (
    <>
      <Banner tone="ok" icon={<IconCheck size={20} />}>
        {done ? (
          <>
            <b className="font-bold">Вы отметили получение.</b> Ждём остальных участников.
          </>
        ) : (
          <>
            <b className="font-bold">Все подтвердили.</b> Переходите к передаче.
          </>
        )}
      </Banner>

      <ExchangeSummary me={me} neighbours={neighbours} />

      {done ? (
        <ChainProgress
          label="Получение отметили"
          value={countReceipts(chain)}
          total={chain.participants.length}
        />
      ) : (
        <Steps steps={steps} />
      )}

      <p className="text-[12.5px] leading-relaxed text-ink-3">
        Цепочка закроется, когда получение отметят все.
      </p>

      <BoardFooter>
        {done ? (
          <p className="text-center text-[12.5px] text-ink-2">
            Свою часть обмена вы выполнили — повторно отмечать не нужно.
          </p>
        ) : (
          <ConfirmReceipt chainId={chain.id} />
        )}
      </BoardFooter>
    </>
  )
}
