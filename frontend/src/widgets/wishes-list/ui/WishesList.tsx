import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getMyItems, itemKeys, WishCard } from '@/entities/item'
import { Button, EmptyState, Notice } from '@/shared/ui'

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
      <EmptyState
        title="Желаний пока нет"
        description="Желание указывают у объявления, когда включают обмен. Чем больше вариантов — тем выше шанс, что цепочка соберётся."
        action={
          <Link to="/">
            <Button variant="dark">К моим объявлениям</Button>
          </Link>
        }
      />
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
