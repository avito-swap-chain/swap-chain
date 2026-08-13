import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { chainKeys, getMyChains, isInboxOffer } from '@/entities/chain'
import { cx } from '@/shared/lib'
import { Screen } from '@/shared/ui'
import { DealsList, PendingDealBanner } from '@/widgets/deals-list'
import { OffersList } from '@/widgets/offers-list'
import { WishesList } from '@/widgets/wishes-list'

type Tab = 'offers' | 'deals' | 'wishes'

const TABS: { key: Tab; label: string }[] = [
  { key: 'offers', label: 'Предложения' },
  { key: 'deals', label: 'Мои обмены' },
  { key: 'wishes', label: 'Желания' },
]

/**
 * Раздел «Обмен» — наш сервис целиком: что подобралось, что уже собралось в цепочки
 * и что человек ищет. Вкладка живёт в адресе (`?tab=wishes`), чтобы на неё можно было дать
 * ссылку и вернуться кнопкой браузера; `replace` — чтобы переключение не копилось в истории.
 *
 * Подобранные варианты — вкладка, а не блок над вкладками. Раньше они стояли выше, и панель
 * разделов уезжала под список: вариантов подбирается сразу несколько, и на четырёх она
 * оказывалась за нижним краем экрана — до «Моих обменов» было не добраться.
 */
export function ExchangePage() {
  const [params, setParams] = useSearchParams()
  const requested = params.get('tab')
  const tab: Tab = TABS.some((t) => t.key === requested) ? (requested as Tab) : 'offers'

  // Счётчик на вкладке — из общего кэша цепочек: список предложений читает те же данные,
  // второго запроса не будет.
  const { data: chains } = useQuery({ queryKey: chainKeys.my(), queryFn: getMyChains })
  const offers = chains?.filter(isInboxOffer).length ?? 0

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3.5 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">Обмен</h1>

        {/* Баннер над вкладками: зовёт в цепочку независимо от того, что сейчас открыто. */}
        <PendingDealBanner />

        <nav aria-label="Разделы обмена" className="flex gap-6 border-b border-line-2">
          {TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setParams(t.key === 'offers' ? {} : { tab: t.key }, { replace: true })}
              aria-pressed={tab === t.key}
              className={cx(
                'relative rounded-sm pb-2.5 text-sm font-semibold outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand',
                tab === t.key ? 'text-ink' : 'text-ink-2',
              )}
            >
              {t.label}
              {/* Счётчик надстрочным, как у вкладок Авито: это количество, а не тревога. */}
              {t.key === 'offers' && offers > 0 && (
                <sup className="ml-0.5 text-[11px] font-bold">{offers}</sup>
              )}
              {/* Полоса активной вкладки у Авито чёрная — в их системе азур означает ссылку. */}
              {tab === t.key && (
                <span className="absolute inset-x-0 -bottom-px h-[2.5px] rounded bg-ink" />
              )}
            </button>
          ))}
        </nav>

        {tab === 'offers' && (
          <>
            <OffersList />
            {offers === 0 && (
              <p className="py-6 text-center text-[13.5px] leading-5 text-ink-2">
                Пока ничего не подобралось. Как только цепочка соберётся, придёт уведомление — вещь
                до этого остаётся у вас.
              </p>
            )}
          </>
        )}

        {tab === 'deals' && <DealsList />}
        {tab === 'wishes' && <WishesList />}
      </div>
    </Screen>
  )
}
