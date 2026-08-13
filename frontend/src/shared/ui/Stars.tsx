import { cx } from '../lib'
import { IconStar } from './icons'

const STARS = [1, 2, 3, 4, 5]

/**
 * Пять звёзд под рейтинг. Не подпись, а картинка: значение рядом набрано цифрой, поэтому
 * звёзды скрыты от экранного читателя — иначе он произносит их пять раз подряд.
 */
export function Stars({ rating = 0, size = 15 }: { rating?: number; size?: number }) {
  const filled = Math.round(rating)

  return (
    <span aria-hidden className="flex gap-0.5">
      {STARS.map((star) => (
        <IconStar
          key={star}
          size={size}
          className={cx(star <= filled ? 'text-attention-dot' : 'text-line')}
        />
      ))}
    </span>
  )
}
