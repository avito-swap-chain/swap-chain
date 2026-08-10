import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getMyItems, ItemCard, ItemStatusLabel, itemKeys, type Item } from '@/entities/item'
import { EnableBarterButton } from '@/features/enable-barter'
import { WithdrawItem } from '@/features/withdraw-item'
import { Button, EmptyState, Notice } from '@/shared/ui'

/**
 * Что можно сделать с объявлением прямо в списке.
 *
 * Вещь в собравшейся цепочке (`reserved`) не трогаем: она уже едет через ПВЗ, снять её
 * нельзя, и бэкенд ответил бы 409. Пока идёт разбор описания, снятие уже доступно —
 * человек мог передумать сразу после публикации, и ждать анализа ради этого незачем.
 */
function actionFor(item: Item) {
  // Обмен ещё не включён — предлагаем включить прямо на объявлении.
  if (item.status === 'idle') return <EnableBarterButton itemId={item.id} />

  // Снятую вещь подписываем, но выход не закрываем: включить обмен обратно — то же
  // действие, что и в первый раз, и без кнопки карточка снова стала бы тупиком.
  if (item.status === 'withdrawn') {
    return (
      <span className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1">
        <ItemStatusLabel status={item.status} />
        <EnableBarterButton itemId={item.id} />
      </span>
    )
  }

  if (item.status === 'reserved') return undefined

  return (
    <span className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1">
      <ItemStatusLabel status={item.status} />
      <WithdrawItem itemId={item.id} />
    </span>
  )
}

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
        <ItemCard key={item.id} item={item} action={actionFor(item)} />
      ))}
    </div>
  )
}
