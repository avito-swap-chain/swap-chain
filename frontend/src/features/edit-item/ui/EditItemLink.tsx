import { Link } from 'react-router-dom'

/**
 * Правка объявления — второстепенное действие рядом со снятием с обмена, поэтому ссылка,
 * а не кнопка: главное действие на карточке одно, и это включение обмена.
 */
export function EditItemLink({ itemId }: { itemId: string }) {
  return (
    <Link
      to={`/items/${itemId}/edit`}
      className="shrink-0 rounded-sm text-[12.5px] font-semibold text-ink-3 outline-offset-2 hover:text-ink-2 focus-visible:outline-2 focus-visible:outline-brand"
    >
      Редактировать
    </Link>
  )
}
