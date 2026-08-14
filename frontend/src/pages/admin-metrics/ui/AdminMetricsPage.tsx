import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { getFunnel, moderationKeys } from '@/entities/moderation'
import { formatDateTime } from '@/shared/lib'
import { Notice, Screen } from '@/shared/ui'

/** Названия причин отказа: бэкенд отдаёт коды, человеку нужны слова. */
const REASON_LABEL: Record<string, string> = {
  declined: 'участник отказался',
  expired: 'время на ответ вышло',
  item_unavailable: 'вещь ушла в другой обмен',
  item_changed: 'вещь изменили',
  item_withdrawn: 'вещь сняли с обмена',
  blocked: 'участники заблокировали друг друга',
  unknown: 'причина не записана',
}

/** Доля в процентах, «—» если знаменателя не было: ноль здесь означал бы «ни разу», а это ложь. */
const percent = (rate?: number | null) =>
  rate === null || rate === undefined ? '—' : `${Math.round(rate * 100)}%`

/** Среднее время до первой цепочки: секунды человеку ни о чём не говорят. */
const duration = (seconds?: number | null) => {
  if (!seconds) return '—'
  if (seconds < 90) return `${Math.round(seconds)} с`
  if (seconds < 5400) return `${Math.round(seconds / 60)} мин`
  return `${(seconds / 3600).toFixed(1)} ч`
}

function Metric({ value, label, hint }: { value: ReactNode; label: string; hint?: string }) {
  return (
    <div className="flex flex-col gap-0.5 border-b border-line py-3.5 last:border-0 sm:border-0">
      <span className="text-[28px] leading-8 font-bold">{value}</span>
      <span className="text-[15px] leading-5">{label}</span>
      {hint && <span className="text-[13px] leading-4 text-ink-2">{hint}</span>}
    </div>
  )
}

/**
 * Воронка обмена — единственный экран, который отвечает не «что происходит с моей вещью»,
 * а «работает ли идея вообще»: доходит ли вещь до цепочки, соглашаются ли участники,
 * доводят ли обмен до конца. Длина цепочки в три участника выбрана ради процента совпадений,
 * и проверяться он должен цифрами.
 *
 * Раздел сотрудника, а не пользователя: считает бэкенд по всей базе, и роль проверяет он же.
 */
export function AdminMetricsPage() {
  const { data, isPending, isError } = useQuery({
    queryKey: moderationKeys.funnel(),
    queryFn: getFunnel,
    // Цифры меняются от действий других людей — с открытым экраном они иначе застынут.
    refetchInterval: (query) => (query.state.error ? false : 30_000),
  })

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">Воронка</h1>

        {isPending && <Notice>Загрузка…</Notice>}
        {isError && (
          <Notice tone="error">
            Не удалось загрузить метрики. Раздел открыт только сотрудникам.
          </Notice>
        )}

        {data && (
          <>
            <p className="text-[13px] leading-4 text-ink-2">
              Снимок на {formatDateTime(data.generatedAt)}
            </p>

            <div className="grid gap-x-8 sm:grid-cols-2 lg:grid-cols-3">
              <Metric
                value={data.eligibleItems}
                label="вещей в обмене"
                hint="ищут цепочку или уже заморожены в ней"
              />
              <Metric
                value={percent(data.itemsWithChainRate)}
                label="из них попали в цепочку"
                hint={`${data.itemsWithChain} вещей хотя бы раз оказались в предложении`}
              />
              <Metric
                value={duration(data.averageTimeToFirstChainSeconds)}
                label="до первой цепочки"
                hint="сколько вещь ждёт первого предложения"
              />
              <Metric
                value={percent(data.acceptanceRate)}
                label="предложений принимают"
                hint={`${data.acceptedChains} из ${data.decidedChains} тех, по которым решение принято`}
              />
              <Metric
                value={data.completedChains}
                label="обменов доведено до конца"
                hint="все участники получили вещи"
              />
              <Metric
                value={percent(data.deliveryCompletionRate)}
                label="доходят до передачи"
                hint="из собравшихся цепочек"
              />
            </div>

            {/* Почему предложения не состоялись — самое полезное здесь: подбор чинится
                по этим причинам, а не по общему проценту. */}
            {data.rejectionReasons.length > 0 && (
              <section className="flex flex-col gap-2 pt-2">
                <h2 className="text-[19px] leading-6 font-bold">Почему варианты не состоялись</h2>

                <ul className="flex flex-col">
                  {[...data.rejectionReasons]
                    .sort((a, b) => b.count - a.count)
                    .map((reason) => (
                      <li
                        key={reason.reason}
                        className="flex items-center justify-between gap-3 border-b border-line py-2.5 text-[15px] leading-5 last:border-0"
                      >
                        <span>{REASON_LABEL[reason.reason] ?? reason.reason}</span>
                        <span className="font-bold">{reason.count}</span>
                      </li>
                    ))}
                </ul>
              </section>
            )}
          </>
        )}
      </div>
    </Screen>
  )
}
