import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { getMyProfile, getProfile, getReviews, userKeys, type Profile } from '@/entities/user'
import { EditProfile } from '@/features/edit-profile'
import { exchangesLabel, reviewsLabel } from '@/shared/lib'
import { Avatar, IconStar, Notice, Screen, ScreenHeader } from '@/shared/ui'

// Месяцы в родительном падеже: `toLocaleDateString` отдаёт именительный, и выходило
// «На Авито с март 2026».
const MONTHS = [
  'января',
  'февраля',
  'марта',
  'апреля',
  'мая',
  'июня',
  'июля',
  'августа',
  'сентября',
  'октября',
  'ноября',
  'декабря',
]

/** Когда человек здесь с: дата регистрации — единственный довод, который отдаёт бэкенд. */
const registered = (iso: string) => {
  const date = new Date(iso)
  return `${MONTHS[date.getMonth()]} ${date.getFullYear()}`
}

/**
 * Профиль пользователя. Один экран на два адреса: `/profile` — свой, с правкой имени
 * и фотографии, `/users/:id` — чужой, только просмотр.
 *
 * Чужой профиль нужен не ради полноты: вещь отдают незнакомому человеку, и «кто он такой»
 * — вопрос, который возникает до согласия. Отсюда на него и ведут ссылки с участников цепочки.
 */
export function ProfilePage({ mine = false }: { mine?: boolean }) {
  const { id = '' } = useParams()
  const navigate = useNavigate()

  const { data, isPending, isError } = useQuery({
    queryKey: mine ? userKeys.me() : userKeys.profile(id),
    queryFn: () => (mine ? getMyProfile() : getProfile(id)),
  })

  return (
    <Screen>
      <ScreenHeader title={mine ? 'Мой профиль' : 'Профиль'} onBack={() => navigate(-1)} />

      <main className="flex flex-1 flex-col gap-4 p-4">
        {isPending && <Notice>Загрузка…</Notice>}
        {isError && <Notice tone="error">Не удалось загрузить профиль</Notice>}

        {data && <ProfileBody profile={data} mine={mine} />}
      </main>
    </Screen>
  )
}

function ProfileBody({ profile, mine }: { profile: Profile; mine: boolean }) {
  return (
    <>
      <div className="flex items-center gap-4">
        <Avatar name={profile.name} src={profile.avatarUrl} className="size-20 text-[28px]" />

        <div className="min-w-0">
          <p className="truncate text-[22px] leading-7 font-bold">{profile.name}</p>
          <p className="text-[13px] text-ink-2">На Авито с {registered(profile.registeredAt)}</p>

          {/* Рейтинг показываем только там, где он настоящий: пока человека никто не оценил,
              его нет вовсе, и «5,0» из воздуха врал бы ровно там, где на него смотрят. */}
          {profile.rating !== undefined && (
            <p className="mt-1 flex items-center gap-1 text-[13px] text-ink-2">
              <IconStar size={15} />
              <b className="font-bold text-ink">{profile.rating.toFixed(1).replace('.', ',')}</b>
              {profile.reviews !== undefined && <span>· {reviewsLabel(profile.reviews)}</span>}
            </p>
          )}

          {/* Число завершённых обменов — довод сильнее рейтинга: он про то, что человек
              доводит сделку до конца, а не про то, как о нём отозвались. */}
          {profile.completedExchanges !== undefined && profile.completedExchanges > 0 && (
            <p className="text-[13px] text-ink-2">{exchangesLabel(profile.completedExchanges)}</p>
          )}
        </div>
      </div>

      {mine && <EditProfile profile={profile} />}

      <Reviews userId={profile.id} count={profile.reviews} />
    </>
  )
}

/** Дата отзыва: день с месяцем, год — только если он не текущий. */
const reviewDate = (iso: string) => {
  const date = new Date(iso)
  const year = date.getFullYear()
  const suffix = year === new Date().getFullYear() ? '' : ` ${year}`
  return `${date.getDate()} ${MONTHS[date.getMonth()]}${suffix}`
}

/**
 * Отзывы о человеке. Появились в контракте 0.8.0 и заменяют собой единственный прежний
 * довод — дату регистрации: обмен сводит незнакомых людей, и «с кем я меняюсь» решается
 * тем, что о нём говорят те, кто уже менялся.
 */
function Reviews({ userId, count }: { userId: string; count?: number }) {
  const { data, isPending, isError } = useQuery({
    queryKey: userKeys.reviews(userId),
    queryFn: () => getReviews(userId),
  })

  if (isPending) return <Notice>Загрузка отзывов…</Notice>
  // Профиль уже показан, и падать целиком из-за отзывов незачем — говорим про них отдельно.
  if (isError) return <Notice tone="error">Не удалось загрузить отзывы</Notice>

  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-[17px] font-bold">
        Отзывы{count !== undefined && count > 0 && <span className="text-ink-2"> · {count}</span>}
      </h2>

      {data.length === 0 ? (
        <p className="text-[13px] text-ink-2">
          Отзывов пока нет — их оставляют соседи по кругу после завершённого обмена.
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {data.map((review) => (
            <li key={review.id} className="flex flex-col gap-1.5 rounded-lg bg-surface-2 p-3">
              <div className="flex items-center gap-2">
                <Avatar name={review.author.name} src={review.author.avatarUrl} className="size-7" />
                <b className="text-[13px] font-bold">{review.author.name}</b>
                <span className="flex items-center gap-0.5 text-[13px] text-ink-2">
                  <IconStar size={14} />
                  {review.rating}
                </span>
                <span className="ml-auto text-[12.5px] text-ink-3">
                  {reviewDate(review.createdAt)}
                </span>
              </div>

              {/* Отзыв без текста — обычное дело: оценку ставят чаще, чем пишут. */}
              {review.text && <p className="text-[13px] leading-relaxed">{review.text}</p>}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
