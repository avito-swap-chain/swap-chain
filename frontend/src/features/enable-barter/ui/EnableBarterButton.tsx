import { Link } from 'react-router-dom'

/**
 * Главный вход в сервис: обмен — не отдельный продукт, а атрибут уже размещённого
 * объявления. Человек ничего не заводит заново, он говорит «эту вещь готов обменять».
 */
export function EnableBarterButton({ itemId }: { itemId: string }) {
  return (
    <Link
      to={`/items/${itemId}/barter`}
      className="shrink-0 rounded-btn bg-brand px-3 py-2 text-[13px] font-bold text-on-brand transition-colors hover:bg-brand-press focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand"
    >
      Готов обменять
    </Link>
  )
}
