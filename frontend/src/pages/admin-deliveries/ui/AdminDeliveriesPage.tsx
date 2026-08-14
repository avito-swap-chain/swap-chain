import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ACTION_LABEL,
  advanceDelivery,
  confirmParticipantReceipt,
  deliveryKeys,
  DeliveryStatusLabel,
  getChain,
  getChains,
  nextStatus,
  type Delivery,
  type DeliveryTransition,
} from '@/entities/delivery'
import type { AdminChainSummary } from '@/shared/api/generated/model'
import { ActionError, Button, EmptyState, Notice, Screen } from '@/shared/ui'

/**
 * Пункт выдачи изнутри: собравшиеся цепочки и путь каждой вещи внутри круга.
 *
 * Через ПВЗ обмен идёт не ради логистики — участники не встречаются лично и не знают друг
 * друга, а вещь всё равно должна дойти. Отсюда и роль сотрудника: он единственный, кто видит
 * вещь физически, поэтому статус двигает он.
 *
 * Список именно цепочками, а не отдельными доставками: вещи ездят кругом на три человека,
 * и «принять» одну имеет смысл только вместе с остальными — круг закрывается, когда каждый
 * получил своё. Плоским списком три передачи одной сделки стояли вперемешку с чужими.
 */
export function AdminDeliveriesPage() {
  const [openId, setOpenId] = useState<number>()

  const { data, isPending, isError } = useQuery({
    queryKey: deliveryKeys.chains(),
    queryFn: getChains,
    // Вещи приносят и забирают весь день: с открытым списком очередь иначе застынет.
    refetchInterval: (query) => (query.state.error ? false : 15_000),
  })

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3.5 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">
          Доставки ПВЗ
        </h1>

        {isPending && <Notice>Загрузка…</Notice>}
        {/* Сюда же приходит 403: роль проверяет бэкенд, и своей проверки на фронте нет —
            вторая, необязательная, только разошлась бы с настоящей. */}
        {isError && (
          <Notice tone="error">
            Не удалось загрузить цепочки. Раздел открыт только сотрудникам пункта выдачи.
          </Notice>
        )}

        {data && data.length === 0 && (
          <EmptyState
            title="Цепочек пока нет"
            description="Здесь появятся собравшиеся цепочки — с вещами, которые участники сдают и забирают в пункте выдачи."
          />
        )}

        {data && data.length > 0 && (
          <ul className="flex flex-col">
            {data.map((chain) => (
              <li key={chain.id}>
                <ChainRow
                  chain={chain}
                  open={chain.id === openId}
                  onToggle={() => setOpenId(chain.id === openId ? undefined : chain.id)}
                />
                {chain.id === openId && <ChainDeliveries chainId={chain.id} />}
              </li>
            ))}
          </ul>
        )}
      </div>
    </Screen>
  )
}

/** Строка очереди: номер круга и сколько его участников уже получили свои вещи. */
function ChainRow({
  chain,
  open,
  onToggle,
}: {
  chain: AdminChainSummary
  open: boolean
  onToggle: () => void
}) {
  const done = chain.receivedCount === chain.participantCount

  return (
    <button
      type="button"
      onClick={onToggle}
      className="flex w-full cursor-pointer items-center justify-between gap-3 border-b border-line py-3.5 text-left last:border-0"
    >
      <span>
        <span className="block text-[15px] leading-5 font-bold">Цепочка №{chain.id}</span>
        <span className="mt-0.5 block text-[13px] leading-4 text-ink-2">
          {chain.participantCount} участника · получили {chain.receivedCount} из{' '}
          {chain.participantCount}
        </span>
      </span>

      <span className="flex items-center gap-3 text-[13px] leading-4 sm:shrink-0">
        {done ? (
          <span className="text-ink-2">Все вещи выданы</span>
        ) : (
          <span className="text-attention">Ход за пунктом выдачи</span>
        )}
        <span className="font-bold text-ink">{open ? 'Свернуть' : 'Открыть'}</span>
      </span>
    </button>
  )
}

/** Передачи внутри круга: от кого кому едет вещь и что сотрудник делает с ней сейчас. */
function ChainDeliveries({ chainId }: { chainId: number }) {
  const { data, isPending, isError } = useQuery({
    queryKey: deliveryKeys.chain(chainId),
    queryFn: () => getChain(chainId),
    refetchInterval: (query) => (query.state.error ? false : 10_000),
  })

  if (isPending) return <Notice>Загрузка…</Notice>
  if (isError) return <Notice tone="error">Не удалось загрузить цепочку</Notice>

  return (
    <ul className="flex flex-col gap-2 border-b border-line bg-line-2 px-3 py-3">
      {data.deliveries.map((delivery) => (
        <li
          key={delivery.id}
          className="flex flex-col gap-2 rounded-card bg-card p-3 sm:flex-row sm:items-center sm:justify-between"
        >
          <span className="min-w-0">
            <span className="block text-[15px] leading-5 font-bold">{delivery.itemTitle}</span>
            {/* Стрелкой, а не «от кого кому»: с бэкенда приходят логины, и в косвенном
                падеже они ломаются («от Даша к Марк»). */}
            <span className="mt-0.5 block text-[13px] leading-4 text-ink-2">
              {delivery.sender.username} → {delivery.recipient.username}
            </span>
          </span>

          <span className="flex items-center gap-4 sm:shrink-0">
            <DeliveryStatusLabel status={delivery.status} />
            <DeliveryAction chainId={chainId} delivery={delivery} />
          </span>
        </li>
      ))}
    </ul>
  )
}

/**
 * Что сотрудник делает с вещью сейчас. До выдачи это шаг доставки, а на выдаче — отметка
 * получения за участника: вещь отдаёт он, и ждать, пока человек подтвердит это из кабинета,
 * незачем — на стойке круг из-за этого и застревал.
 */
function DeliveryAction({ chainId, delivery }: { chainId: number; delivery: Delivery }) {
  const queryClient = useQueryClient()
  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: deliveryKeys.chain(chainId) })
    queryClient.invalidateQueries({ queryKey: deliveryKeys.chains() })
  }

  const advance = useMutation({
    mutationFn: (to: DeliveryTransition) => advanceDelivery(delivery.id, to),
    onSuccess: refresh,
  })

  const receipt = useMutation({
    mutationFn: () => confirmParticipantReceipt(chainId, delivery.recipient.id),
    onSuccess: refresh,
  })

  if (delivery.status === 'RECEIVED') return null

  const handOut = delivery.status === 'IN_DELIVERY'
  const next = nextStatus(delivery.status)
  const pending = advance.isPending || receipt.isPending

  return (
    <span className="flex flex-col items-end gap-1">
      <Button
        disabled={pending}
        onClick={() => (handOut ? receipt.mutate() : next && advance.mutate(next))}
      >
        {handOut ? 'Выдать и отметить получение' : next && ACTION_LABEL[next]}
      </Button>

      {/* Отказ здесь возможен и без ошибки сотрудника: ту же вещь мог провести коллега. */}
      <ActionError error={advance.error ?? receipt.error} />
    </span>
  )
}
