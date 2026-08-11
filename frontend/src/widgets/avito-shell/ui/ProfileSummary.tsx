import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getCurrentUser, sessionKeys } from '@/shared/model/session'
import { asset, cx, reviewsLabel } from '@/shared/lib'
import { IconStar } from '@/shared/ui'

const STARS = [1, 2, 3, 4, 5]

/**
 * Шапка кабинета Авито: крупный аватар, имя, рейтинг со звёздами и отзывы — и только
 * под чертой меню разделов. Это не украшение: кабинет открывается с ответа на вопрос
 * «кто я здесь», а в демо с переключением персон он ещё и показывает, за кого смотрят.
 *
 * Рейтинга и фотографии у пользователя с бэкенда нет — тогда остаются имя и инициал.
 * Подставлять сюда чужие звёзды нельзя: рейтинг в обмене это довод при выборе, и
 * выдуманный он врёт ровно там, где на него смотрят.
 */
export function ProfileSummary({ className }: { className?: string }) {
  const { data: user } = useQuery({ queryKey: sessionKeys.current(), queryFn: getCurrentUser })
  if (!user) return null

  const filled = user.rating === undefined ? 0 : Math.round(user.rating)

  return (
    <div className={cx('border-b border-line pb-4', className)}>
      {user.avatarUrl ? (
        <img
          src={asset(user.avatarUrl)}
          alt=""
          className="size-[140px] rounded-full object-cover"
          width={140}
          height={140}
        />
      ) : (
        <span
          aria-hidden
          className="grid size-[140px] place-items-center rounded-full bg-line text-[48px] font-bold text-ink-3"
        >
          {user.name.slice(0, 1).toUpperCase()}
        </span>
      )}

      {/* Имя ведёт в профиль: там его правят и загружают фотографию. */}
      <Link
        to="/profile"
        className="mt-4 block rounded-sm text-[24px] leading-7 font-bold outline-offset-4 hover:text-brand focus-visible:outline-2 focus-visible:outline-brand"
      >
        {user.name}
      </Link>

      {user.rating !== undefined && (
        <div className="mt-1.5 flex items-center gap-1.5">
          <span className="text-[15px] leading-5 font-bold">
            {user.rating.toLocaleString('ru-RU')}
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
          {user.reviews !== undefined && (
            <span className="text-[15px] leading-5 text-brand">{reviewsLabel(user.reviews)}</span>
          )}
        </div>
      )}
    </div>
  )
}
