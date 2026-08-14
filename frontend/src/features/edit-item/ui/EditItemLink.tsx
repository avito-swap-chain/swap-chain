import { Link } from 'react-router-dom'
import { IconPencil } from '@/shared/ui'

/**
 * Правка объявления — карандаш, как в списке объявлений Авито: на карточке слово занимало
 * строку наравне со статусом и читалось третьим равноправным действием, хотя главное здесь
 * одно — включение обмена. Значок узнаётся мгновенно, а подпись остаётся экранным читалкам.
 *
 * За карандашом теперь и снятие с обмена: оба действия про одно объявление, и опасное
 * из них незачем держать на карточке, где его задевают мимоходом.
 */
export function EditItemLink({ itemId }: { itemId: string }) {
  return (
    <Link
      to={`/items/${itemId}/edit`}
      aria-label="Редактировать"
      title="Редактировать"
      className="grid size-8 shrink-0 place-items-center rounded-full text-ink-3 outline-offset-2 hover:bg-line-2 hover:text-ink focus-visible:outline-2 focus-visible:outline-brand"
    >
      <IconPencil size={18} />
    </Link>
  )
}
