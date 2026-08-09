import { usePersonaStore, PERSONAS } from '@/shared/model/persona'
import { cx } from '@/shared/lib'
import { IconStar } from '@/shared/ui'

const STARS = [1, 2, 3, 4, 5]

/** «24 отзыва», «41 отзыв», «17 отзывов» — без склонения подпись выглядит машинной. */
const reviewsLabel = (count: number): string => {
  const tail = count % 100
  if (tail > 4 && tail < 21) return `${count} отзывов`
  switch (count % 10) {
    case 1:
      return `${count} отзыв`
    case 2:
    case 3:
    case 4:
      return `${count} отзыва`
    default:
      return `${count} отзывов`
  }
}

/**
 * Шапка кабинета Авито: крупный аватар, имя, рейтинг со звёздами и отзывы — и только
 * под чертой меню разделов. Это не украшение: кабинет открывается с ответа на вопрос
 * «кто я здесь», а в демо с переключением персон он ещё и показывает, за кого смотрят.
 */
export function ProfileSummary({ className }: { className?: string }) {
  const personaId = usePersonaStore((state) => state.personaId)
  const persona = PERSONAS.find((item) => item.id === personaId) ?? PERSONAS[0]
  const filled = Math.round(persona.rating)

  return (
    <div className={cx('border-b border-line pb-4', className)}>
      <img
        src={persona.avatarUrl}
        alt=""
        className="size-[140px] rounded-full object-cover"
        width={140}
        height={140}
      />

      <p className="mt-4 text-[24px] leading-7 font-bold">{persona.name}</p>

      <div className="mt-1.5 flex items-center gap-1.5">
        <span className="text-[15px] leading-5 font-bold">
          {persona.rating.toLocaleString('ru-RU')}
        </span>
        <span aria-hidden className="flex gap-0.5">
          {STARS.map((star) => (
            <IconStar
              key={star}
              size={15}
              className={star <= filled ? 'text-attention-dot' : 'text-line'}
            />
          ))}
        </span>
        <span className="text-[15px] leading-5 text-brand">{reviewsLabel(persona.reviews)}</span>
      </div>
    </div>
  )
}
