import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getMyProfile, userKeys } from '@/entities/user'
import { getCurrentUser, sessionKeys } from '@/shared/model/session'
import { asset, cx, reviewsLabel } from '@/shared/lib'
import { IconCamera, Stars } from '@/shared/ui'

/**
 * Шапка кабинета Авито: крупный аватар, имя, рейтинг со звёздами и отзывы — и только
 * под чертой меню разделов. Это не украшение: кабинет открывается с ответа на вопрос
 * «кто я здесь», а в демо с переключением персон он ещё и показывает, за кого смотрят.
 *
 * Рейтинга и фотографии у пользователя может не быть — тогда остаются имя и инициал.
 * Подставлять сюда чужие звёзды нельзя: рейтинг в обмене это довод при выборе, и
 * выдуманный он врёт ровно там, где на него смотрят.
 *
 * Данные берём из профиля, а не только из сессии: сессия отдаёт имя, телефон и роль,
 * а фотография с рейтингом живут в профиле. Пока колонка читала одну сессию, человек
 * видел свой аватар на странице профиля и серый инициал слева — на одном экране.
 */
export function ProfileSummary({ className }: { className?: string }) {
  const { data: session } = useQuery({ queryKey: sessionKeys.current(), queryFn: getCurrentUser })
  const { data: profile } = useQuery({
    queryKey: userKeys.me(),
    queryFn: getMyProfile,
    enabled: Boolean(session),
  })

  const user = profile ?? session
  if (!user) return null

  return (
    /* Размеры с их кабинета: аватар 100, имя 21/26, рейтинг 18 полужирным и звёзды 20 —
       у нас всё это было крупнее, и колонка перевешивала содержимое страницы. */
    <div className={cx('border-b border-line pb-3.5', className)}>
      <div className="relative w-[100px]">
        {user.avatarUrl ? (
          <img
            src={asset(user.avatarUrl)}
            alt=""
            className="size-[100px] rounded-full object-cover"
            width={100}
            height={100}
          />
        ) : (
          <span
            aria-hidden
            className="grid size-[100px] place-items-center rounded-full bg-line text-[34px] font-bold text-ink-3"
          >
            {user.name.slice(0, 1).toUpperCase()}
          </span>
        )}

        {/* Кнопка смены фотографии на углу аватара — как у Авито. Правка живёт в профиле. */}
        <Link
          to="/profile"
          aria-label="Изменить фотографию"
          title="Изменить фотографию"
          className="absolute right-0 bottom-0 grid size-7 place-items-center rounded-full bg-line text-ink outline-offset-2 hover:bg-line-2 focus-visible:outline-2 focus-visible:outline-brand"
        >
          <IconCamera size={16} />
        </Link>
      </div>

      {/* Имя ведёт в профиль: там его правят и загружают фотографию. */}
      <Link
        to="/profile"
        className="mt-3 block rounded-sm text-[21px] leading-[26px] font-bold outline-offset-4 hover:text-brand focus-visible:outline-2 focus-visible:outline-brand"
      >
        {user.name}
      </Link>

      {user.rating !== undefined && (
        <div className="mt-1 flex items-center gap-2">
          <span className="text-[18px] leading-6 font-bold">
            {user.rating.toLocaleString('ru-RU')}
          </span>
          <Stars rating={user.rating} size={20} />
          {user.reviews !== undefined && (
            <span className="text-[15px] leading-[22px] text-brand-hover">
              {reviewsLabel(user.reviews)}
            </span>
          )}
        </div>
      )}
    </div>
  )
}
