import { countReceipts, ExchangeSummary } from '@/entities/chain'
import { DeliveryStatusLabel } from '@/entities/delivery'
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

  // Где физически обе вещи: моя едет соседу-получателю (для него она входящая), а мне везут
  // вещь от соседа-отдающего. С контракта 0.8.0 это приходит в самой цепочке, и стадия
  // передачи больше не молчит до чьей-нибудь отметки о получении.
  const mineOnTheWay = receiver.incomingDelivery
  const comingToMe = me.incomingDelivery

  // Свою вещь пользователь сдал, как только ПВЗ её принял. Пока статуса доставки нет
  // (старый бэкенд или моки), остаётся прежний признак: сосед отметил получение.
  const handedOver = mineOnTheWay
    ? mineOnTheWay !== 'AWAITING_PVZ'
    : receiver.receiptConfirmed
  const inDelivery = mineOnTheWay === 'IN_DELIVERY' || mineOnTheWay === 'RECEIVED'
  const readyToPickUp = comingToMe === 'IN_DELIVERY' || comingToMe === 'RECEIVED'

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
      state: inDelivery ? 'done' : handedOver ? 'current' : 'todo',
      content: <>Дождитесь, пока ПВЗ передаст её в доставку</>,
    },
    {
      state: readyToPickUp ? 'current' : 'todo',
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

      {/* Где обе вещи прямо сейчас. Без этого стадия передачи выглядела застывшей: человек
          сдал вещь и не понимал, дошла ли она и едет ли к нему встречная. */}
      {(mineOnTheWay || comingToMe) && (
        <dl className="flex flex-col gap-2 rounded-lg bg-surface-2 p-3">
          {mineOnTheWay && (
            <div className="flex items-center justify-between gap-3">
              <dt className="text-[12.5px] text-ink-2">Ваша вещь</dt>
              <dd>
                <DeliveryStatusLabel status={mineOnTheWay} />
              </dd>
            </div>
          )}
          {comingToMe && (
            <div className="flex items-center justify-between gap-3">
              <dt className="text-[12.5px] text-ink-2">Вам везут</dt>
              <dd>
                <DeliveryStatusLabel status={comingToMe} direction="incoming" />
              </dd>
            </div>
          )}
        </dl>
      )}

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
