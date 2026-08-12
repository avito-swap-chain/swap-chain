import { useQuery } from '@tanstack/react-query'
import {
  chainKeys,
  countVariantsWithItem,
  findMe,
  getMyChains,
  isOpenOffer,
  type Chain,
} from '@/entities/chain'
import { getMyItems, itemKeys } from '@/entities/item'
import { OfferCard } from './OfferCard'
import { isBackendConnected } from '@/shared/config/backend'
import { Notice } from '@/shared/ui'

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
  // Вещи читаем из общего кэша ради одного признака: ждём ли мы сейчас бэкенд. Пока вещь
  // разбирается или ищет обмен, ответ придёт не по нашему действию, а сам.
  const { data: items } = useQuery({
    queryKey: itemKeys.my(),
    queryFn: getMyItems,
    refetchInterval: (query) =>
      isBackendConnected &&
      query.state.data?.some((item) => item.status === 'analyzing' || item.status === 'searching')
        ? 15_000
        : false,
  })
  const awaitingBackend = items?.some(
    (item) => item.status === 'analyzing' || item.status === 'searching',
  )

  const { data, isError } = useQuery({
    queryKey: chainKeys.my(),
    queryFn: getMyChains,
    // Обновления приносит поток событий, но полагаться только на него нельзя: не пришло
    // событие — экран навсегда остаётся с «Поиск обмена», хотя цепочка уже собрана. Пока
    // человек чего-то ждёт, страхуемся редким опросом; событие по-прежнему обновляет сразу.
    // На моках ответы остальных участников появляются по таймеру, там опрос частый.
    refetchInterval: (query) => {
      const formed = query.state.data?.some((chain) => chain.status === 'formed')
      if (!isBackendConnected) return formed ? 2000 : false
      return awaitingBackend || formed ? 15_000 : false
    },
  })

  // Молчаливое исчезновение раздела читалось бы как «сервис ничего не подобрал» — самый
  // обидный вывод, который человек может сделать из упавшего запроса.
  if (isError) return <Notice tone="error">Не удалось загрузить предложения обмена</Notice>

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
