import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { getMyProfile, getProfile, userKeys, type Profile } from '@/entities/user'
import { EditProfile } from '@/features/edit-profile'
import { reviewsLabel } from '@/shared/lib'
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

          {/* Рейтинг показываем только там, где он настоящий: у бэкенда его нет, и заведён
              задачей. Выдуманные звёзды врут ровно в том месте, где на них смотрят. */}
          {profile.rating !== undefined && (
            <p className="mt-1 flex items-center gap-1 text-[13px] text-ink-2">
              <IconStar size={15} />
              <b className="font-bold text-ink">{profile.rating.toFixed(1).replace('.', ',')}</b>
              {profile.reviews !== undefined && <span>· {reviewsLabel(profile.reviews)}</span>}
            </p>
          )}
        </div>
      </div>

      {mine && <EditProfile profile={profile} />}
    </>
  )
}
