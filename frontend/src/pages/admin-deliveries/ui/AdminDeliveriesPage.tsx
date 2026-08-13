import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ACTION_LABEL,
  advanceDelivery,
  deliveryKeys,
  DeliveryStatusLabel,
  getDeliveries,
  nextStatus,
  type Delivery,
  type DeliveryTransition,
} from '@/entities/delivery'
import { Button, Card, EmptyState, Notice, Screen } from '@/shared/ui'

/**
 * Кнопка получает следующий шаг готовым, а не вычисляет его сама: строка всё равно должна
 * знать, есть ли он, — иначе у полученной вещи пришлось бы рисовать кнопку и тут же прятать.
 */
function AdvanceButton({ delivery, to }: { delivery: Delivery; to: DeliveryTransition }) {
  const queryClient = useQueryClient()
  const { mutate, isPending, error } = useMutation({
    mutationFn: () => advanceDelivery(delivery.id, to),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: deliveryKeys.list() }),
  })

  return (
    <div className="flex flex-col items-end gap-1">
      <Button disabled={isPending} onClick={() => mutate()}>
        {ACTION_LABEL[to]}
      </Button>
      {/* Отказ здесь возможен и без ошибки сотрудника: ту же вещь мог провести коллега. */}
      {error && <p className="text-[13px] leading-4 text-stop">{error.message}</p>}
    </div>
  )
}

function DeliveryRow({ delivery }: { delivery: Delivery }) {
  const next = nextStatus(delivery.status)

  return (
    <Card padded className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0">
        <p className="text-[15px] leading-5 font-bold">{delivery.itemTitle}</p>
        {/* Стрелкой, а не «от кого кому»: с бэкенда приходят логины, и в косвенном падеже
            они ломаются («от Даша к Марк»). Направление передачи стрелка показывает точнее. */}
        <p className="mt-0.5 text-[13px] leading-4 text-ink-2">
          {delivery.sender.username} → {delivery.recipient.username} · цепочка №{delivery.chainId}
        </p>
      </div>

      <div className="flex items-center gap-4 sm:shrink-0">
        <DeliveryStatusLabel status={delivery.status} />
        {next && <AdvanceButton delivery={delivery} to={next} />}
      </div>
    </Card>
  )
}

/**
 * Пункт выдачи изнутри: вещи собравшихся цепочек и их путь от отправителя к получателю.
 * Через ПВЗ обмен идёт не ради логистики — участники не встречаются лично и не знают друг
 * друга, а вещь всё равно должна дойти. Отсюда и роль сотрудника: он единственный, кто
 * видит вещь физически, поэтому статус двигает он, а не участники цепочки.
 */
export function AdminDeliveriesPage() {
  const { data, isPending, isError } = useQuery({
    queryKey: deliveryKeys.list(),
    queryFn: getDeliveries,
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
            Не удалось загрузить доставки. Раздел открыт только сотрудникам пункта выдачи.
          </Notice>
        )}

        {data && data.length === 0 && (
          <EmptyState
            title="Доставок пока нет"
            description="Здесь появятся вещи собравшихся цепочек — по одной на каждого участника."
          />
        )}

        {data && data.length > 0 && (
          <div className="flex flex-col gap-2">
            {data.map((delivery) => (
              <DeliveryRow key={delivery.id} delivery={delivery} />
            ))}
          </div>
        )}
      </div>
    </Screen>
  )
}
