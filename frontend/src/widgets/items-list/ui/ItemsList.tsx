import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getMyItems, ItemCard, itemKeys } from '@/entities/item'
import { EnableBarterButton } from '@/features/enable-barter'
import { Button, EmptyState, Notice } from '@/shared/ui'

export function ItemsList() {
  const { data, isPending, isError } = useQuery({
    queryKey: itemKeys.my(),
    queryFn: getMyItems,
  })

  if (isPending) return <Notice>Загрузка…</Notice>
  if (isError) return <Notice tone="error">Не удалось загрузить объявления</Notice>
  if (data.length === 0) {
    return (
      <EmptyState
        illustration="items"
        title="Объявлений пока нет"
        description="Разместите первое — а потом включите у него обмен, и сервис подберёт цепочку."
        action={
          <Link to="/items/new">
            <Button variant="dark">Разместить объявление</Button>
          </Link>
        }
      />
    )
  }

  return (
    // Две колонки только с `xl`: карточка — компактная строка с кнопкой справа,
    // и уже ниже ~470px на колонку заголовку не остаётся места. Третья колонка
    // в контейнер страницы не влезает по той же причине.
    // `grid-cols-1` задаётся явно: неявная колонка грида тянется по max-content,
    // и длинный заголовок выносил бы карточку за край экрана.
    <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
      {data.map((item) => (
        <ItemCard
          key={item.id}
          item={item}
          // Обмен ещё не включён — предлагаем включить прямо на объявлении.
          action={item.status === 'idle' ? <EnableBarterButton itemId={item.id} /> : undefined}
        />
      ))}
    </div>
  )
}
