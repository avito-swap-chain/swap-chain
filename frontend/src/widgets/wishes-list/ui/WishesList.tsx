import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getMyItems, itemKeys, WishCard } from '@/entities/item'
import { Notice } from '@/shared/ui'

/** Что пользователь ищет: желания берутся из его же вещей (wish — поле вещи). */
export function WishesList() {
  const { data, isPending, isError } = useQuery({
    queryKey: itemKeys.my(),
    queryFn: getMyItems,
  })

  if (isPending) return <Notice>Загрузка…</Notice>
  if (isError) return <Notice tone="error">Не удалось загрузить желания</Notice>

  const wanted = data.filter((item) => item.wish.length > 0)
  if (wanted.length === 0) {
    // Желание указывают у объявления, когда включают обмен, — туда и ведём из пустого списка.
    return (
      <div className="flex flex-col items-center">
        <Notice>Желаний пока нет. Укажите их у объявления, включив обмен.</Notice>
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
      {wanted.map((item) => (
        <WishCard key={item.id} item={item} />
      ))}
    </div>
  )
}
