import { useSearchParams } from 'react-router-dom'
import { cx } from '@/shared/lib'
import { Screen } from '@/shared/ui'
import { DealsList, PendingDealBanner } from '@/widgets/deals-list'
import { OffersList } from '@/widgets/offers-list'
import { WishesList } from '@/widgets/wishes-list'

type Tab = 'deals' | 'wishes'

const TABS: { key: Tab; label: string }[] = [
  { key: 'deals', label: 'Мои обмены' },
  { key: 'wishes', label: 'Желания' },
]

/**
 * Раздел «Обмен» — наш сервис целиком: что уже собралось в цепочки и что человек ищет.
 * Вкладка живёт в адресе (`?tab=wishes`), чтобы на неё можно было дать ссылку и вернуться
 * кнопкой браузера; `replace` — чтобы переключение вкладок не копилось в истории.
 */
export function ExchangePage() {
  const [params, setParams] = useSearchParams()
  const requested = params.get('tab')
  const tab: Tab = TABS.some((t) => t.key === requested) ? (requested as Tab) : 'deals'

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3.5 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">Обмен</h1>

        {/* Баннер над вкладками: зовёт в цепочку независимо от того, что сейчас открыто. */}
        <PendingDealBanner />

        {/* Подобранные варианты стоят над вкладками и первыми: ответ по ним — то, ради чего
            человек сюда пришёл, и вариантов у него сразу несколько. */}
        <OffersList />

        <nav aria-label="Разделы обмена" className="flex gap-6 border-b border-line-2">
          {TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setParams(t.key === 'deals' ? {} : { tab: t.key }, { replace: true })}
              aria-pressed={tab === t.key}
              className={cx(
                'relative rounded-sm pb-2.5 text-sm font-semibold outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand',
                tab === t.key ? 'text-ink' : 'text-ink-2',
              )}
            >
              {t.label}
              {/* Полоса активной вкладки у Авито чёрная — в их системе азур означает ссылку. */}
              {tab === t.key && (
                <span className="absolute inset-x-0 -bottom-px h-[2.5px] rounded bg-ink" />
              )}
            </button>
          ))}
        </nav>

        {tab === 'deals' ? <DealsList /> : <WishesList />}
      </div>
    </Screen>
  )
}
