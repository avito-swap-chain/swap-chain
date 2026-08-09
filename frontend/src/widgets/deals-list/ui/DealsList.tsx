import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ChainCard, chainKeys, getMyChains, isOpenOffer } from '@/entities/chain'
import { Notice } from '@/shared/ui'
import { sortByUrgency } from '../lib/sortByUrgency'

export function DealsList() {
  const { data, isPending, isError } = useQuery({
    queryKey: chainKeys.my(),
    queryFn: getMyChains,
  })

  if (isPending) return <Notice>Загрузка…</Notice>
  if (isError) return <Notice tone="error">Не удалось загрузить обмены</Notice>

  // Предложения, по которым человек ещё не ответил, и отменившиеся варианты живут в блоке
  // «Варианты обмена» — здесь только то, во что человек уже вошёл, и история.
  const deals = data.filter((chain) => !isOpenOffer(chain) && chain.status !== 'cancelled')
  if (deals.length === 0) {
    // Обмены заводятся не здесь, а у объявления, — из пустого списка туда и уводим.
    return (
      <div className="flex flex-col items-center">
        <Notice>Обменов пока нет. Включите обмен у объявления — сервис подберёт цепочку.</Notice>
        <Link
          to="/"
          className="rounded-sm text-[13px] font-semibold text-brand outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand"
        >
          К моим объявлениям
        </Link>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
      {sortByUrgency(deals).map((chain) => (
        <Link
          key={chain.id}
          to={`/exchange/${chain.id}`}
          className="rounded-card outline-offset-2 focus-visible:outline-2 focus-visible:outline-brand"
        >
          <ChainCard chain={chain} />
        </Link>
      ))}
    </div>
  )
}
