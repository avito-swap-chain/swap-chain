import { useQuery } from '@tanstack/react-query'
import {
  chainKeys,
  countVariantsWithItem,
  findMe,
  getMyChains,
  isOpenOffer,
  type Chain,
} from '@/entities/chain'
import { OfferCard } from './OfferCard'
import { isBackendConnected } from '@/shared/config/backend'

/**
 * Что человеку нужно увидеть в блоке предложений: варианты, ждущие ответа, и те, что
 * отменились, — вторые остаются, пока их не объяснили, иначе вариант просто исчезнет.
 */
const isInbox = (chain: Chain): boolean => isOpenOffer(chain) || chain.status === 'cancelled'

/** Отменённые уходят вниз: сначала то, на что можно ответить. */
const answerableFirst = (a: Chain, b: Chain): number =>
  Number(a.status === 'cancelled') - Number(b.status === 'cancelled')

/**
 * Подобранные варианты обмена. Подбор параллельный: цепочек человеку предлагают сразу
 * несколько, и решение по каждой принимается прямо здесь — экран цепочки нужен только тем,
 * кто хочет разобраться подробнее.
 */
export function OffersList() {
  const { data } = useQuery({
    queryKey: chainKeys.my(),
    queryFn: getMyChains,
    // С бэкендом обновления приносит поток событий — опрос остаётся только для моков,
    // где ответы остальных участников появляются по таймеру.
    refetchInterval: (query) =>
      !isBackendConnected && query.state.data?.some((chain) => chain.status === 'formed')
        ? 2000
        : false,
  })

  const offers = data?.filter(isInbox).sort(answerableFirst) ?? []
  if (offers.length === 0) return null

  return (
    <section className="flex flex-col gap-3">
      {/* Механику объясняем один раз здесь: в карточках она превращалась в повтор одного
          и того же абзаца столько раз, сколько вариантов подобрано. */}
      <div>
        <h2 className="text-[15px] font-bold">Варианты обмена</h2>
        <p className="text-[12.5px] text-ink-2">
          Состоится один — тот, что подойдёт всем участникам. Вещь пока остаётся у вас.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        {offers.map((chain) => (
          <OfferCard
            key={chain.id}
            chain={chain}
            variants={countVariantsWithItem(data ?? [], findMe(chain)?.givesItem.id ?? '')}
          />
        ))}
      </div>
    </section>
  )
}
