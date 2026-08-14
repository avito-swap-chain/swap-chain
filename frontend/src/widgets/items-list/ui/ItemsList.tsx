import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getMyItems, ItemCard, ItemStatusLabel, itemKeys, type Item } from '@/entities/item'
import { EditItemLink } from '@/features/edit-item'
import { EnableBarterButton } from '@/features/enable-barter'
import { ResolveWishCategory } from '@/features/resolve-wish-category'
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

  // Раздел желания не определился — вещь стоит вне подбора, и пока это не решено,
  // остальные действия над ней второстепенны.
  if (item.status === 'needs_category' && item.pendingWishes?.length) {
    return (
      <span className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1">
        <ItemStatusLabel status={item.status} />
        <ResolveWishCategory itemId={item.id} wishes={item.pendingWishes} />
      </span>
    )
  }

  // Вещь в собравшейся цепочке и вещь, уже уехавшая новому владельцу, одинаково
  // неприкосновенны: править и снимать нечего, а подпись статуса карточка нарисует сама.
  if (item.status === 'reserved' || item.status === 'exchanged') return undefined

  return (
    <span className="flex flex-wrap items-center justify-end gap-x-2 gap-y-1">
      <ItemStatusLabel status={item.status} />
      {/* Пока вещь в подборе, описание и желание правятся без снятия с обмена: снятие
          отменило бы предложения с ней у других людей, а это не то же самое, что правка.
          Само снятие живёт за карандашом — на карточке ему не место. */}
      <EditItemLink itemId={item.id} />
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
            <Button>Разместить объявление</Button>
          </Link>
        }
      />
    )
  }

  return (
    // Одна колонка строк, как в их кабинете: объявления идут сверху вниз и разделены
    // линией. Двух колонок у Авито нет — и правильно, в них заголовок сжимался до
    // «Монитор L…» на первом же длинном названии.
    <div className="flex flex-col">
      {data.map((item) => (
        <ItemCard key={item.id} item={item} action={actionFor(item)} />
      ))}
    </div>
  )
}
