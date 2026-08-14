import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { chainKeys, getMyChains, needsMyAction } from '@/entities/chain'
import { getUserItems, ItemCard, itemKeys } from '@/entities/item'
import { getMyProfile, getProfile, getReviews, userKeys, type Profile } from '@/entities/user'
import { BlockUser } from '@/features/block-user'
import { EditProfile } from '@/features/edit-profile'
import { ReportUser } from '@/features/report-user'
import { exchangesLabel, reviewsLabel } from '@/shared/lib'
import {
  Avatar,
  Counter,
  IconNotifications,
  IconBan,
  IconBox,
  IconChat,
  IconPencil,
  IconStar,
  IconSwap,
  Notice,
  Screen,
  ScreenHeader,
  Stars,
  Tile,
  TileGroup,
  TileRow,
} from '@/shared/ui'

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
 *
 * Раскладка — мобильного кабинета Авито: крупный аватар, имя, две плитки-метрики и разделы
 * группами строк на серой подложке. Разделов у нас меньше — денег в продукте нет, — но
 * каждая строка ведёт туда, где что-то есть.
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
      {/* Свой профиль — раздел кабинета, в меню он есть, и возвращаться из него некуда:
          кнопка «назад» тут увела бы на случайный прошлый экран. У чужого она нужна —
          туда попадают из цепочки или переписки. */}
      <ScreenHeader
        title={mine ? 'Управление профилем' : 'Профиль'}
        onBack={mine ? undefined : () => navigate(-1)}
      />

      <main className="flex flex-1 flex-col gap-5 p-4">
        {isPending && <Notice>Загрузка…</Notice>}
        {isError && <Notice tone="error">Не удалось загрузить профиль</Notice>}

        {data && <ProfileBody profile={data} mine={mine} />}
      </main>
    </Screen>
  )
}

function ProfileBody({ profile, mine }: { profile: Profile; mine: boolean }) {
  const [editing, setEditing] = useState(false)

  return (
    <>
      <div className="flex flex-col gap-3">
        {/* Кольцо вокруг аватара — как у выбранного профиля в кабинете Авито: экран
            открывается с ответа «кто я здесь», а в демо ещё и «за кого смотрят». */}
        <Avatar
          name={profile.name}
          src={profile.avatarUrl}
          className={
            mine ? 'size-20 text-[28px] ring-3 ring-brand ring-offset-3' : 'size-20 text-[28px]'
          }
        />

        <div>
          <p className="text-[22px] leading-7 font-bold">{profile.name}</p>
          <p className="text-[13px] text-ink-2">На Авито с {registered(profile.registeredAt)}</p>
        </div>
      </div>

      {/* Две метрики в ряд. Рейтинг показываем даже нулевым: подпись «Нет отзывов» говорит
          прямо, что за ним ничего не стоит, — а пустое место на его месте выглядит поломкой. */}
      <div className="grid grid-cols-2 gap-2">
        <Tile
          value={
            <>
              {(profile.rating ?? 0).toFixed(1).replace('.', ',')}
              <Stars rating={profile.rating} />
            </>
          }
          label={profile.reviews ? reviewsLabel(profile.reviews) : 'Нет отзывов'}
        />

        {/* Число завершённых обменов — довод сильнее рейтинга: он про то, что человек
            доводит сделку до конца, а не про то, как о нём отозвались. */}
        <Tile
          value={profile.completedExchanges ?? 0}
          label={
            profile.completedExchanges
              ? exchangesLabel(profile.completedExchanges).replace(/^\d+\s/, '')
              : 'Обменов пока нет'
          }
        />
      </div>

      {mine && <Cabinet onEdit={() => setEditing((open) => !open)} editing={editing} />}

      {mine && editing && <EditProfile profile={profile} />}

      {/* Вещи человека — довод не слабее рейтинга: прежде чем отдать своё, смотрят,
          с чем он вообще пришёл в обмен. У себя список не дублируем — он и есть кабинет. */}
      {!mine && <UserItems userId={profile.id} />}

      <Reviews userId={profile.id} count={profile.reviews} />

      {/* Защита — в чужом профиле и внизу, как «Пожаловаться» у Авито: сначала человек
          читает, кто перед ним, и только потом решает, что с ним делать. */}
      {!mine && (
        <section className="flex flex-col gap-2">
          <h2 className="text-[19px] leading-6 font-bold">Безопасность</h2>
          <ReportUser userId={profile.id} />
          <BlockUser userId={profile.id} name={profile.name} />
        </section>
      )}
    </>
  )
}

/**
 * Разделы кабинета строками — как «Инструменты» у Авито. Только на узких экранах: там меню
 * свёрнуто в нижнюю панель, и профиль остаётся единственным местом, откуда видно всё сразу.
 * На десктопе те же пункты стоят в левой колонке, и второй их список рядом — просто дубль;
 * у Авито в профиле его тоже нет.
 */
function Cabinet({ onEdit, editing }: { onEdit: () => void; editing: boolean }) {
  const { data } = useQuery({ queryKey: chainKeys.my(), queryFn: getMyChains })
  const waiting = data?.filter(needsMyAction).length ?? 0

  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-[19px] leading-6 font-bold lg:hidden">Кабинет</h2>

      {/* Разделы дублируют меню оболочки, поэтому на десктопе их нет — там меню в колонке
          слева. А правка профиля не дубль: это действие, и живёт оно только здесь. */}
      <TileGroup className="lg:hidden">
        <TileRow icon={<IconBox size={19} />} to="/">
          Мои объявления
        </TileRow>

        <TileRow
          icon={<IconSwap size={19} />}
          to="/exchange"
          trailing={waiting > 0 && <Counter title="Ждут вашего действия">{waiting}</Counter>}
        >
          Обмены
        </TileRow>

        {/* На телефоне меню кабинета нет, а в нижнюю панель отзывы не влезают —
            поэтому вход в раздел здесь, как в мобильном профиле Авито. */}
        <TileRow icon={<IconStar size={19} />} to="/reviews">
          Мои отзывы
        </TileRow>

        <TileRow icon={<IconChat size={19} />} to="/messages">
          Сообщения
        </TileRow>

        <TileRow icon={<IconNotifications size={19} />} to="/notifications">
          Уведомления
        </TileRow>

        <TileRow icon={<IconBan size={19} />} to="/blocked">
          Чёрный список
        </TileRow>
      </TileGroup>

      <TileGroup>
        <TileRow icon={<IconPencil size={19} />} onClick={onEdit}>
          {editing ? 'Свернуть правку профиля' : 'Изменить имя и фотографию'}
        </TileRow>
      </TileGroup>
    </section>
  )
}

/** Вещи человека на обмене. Показываем первый десяток: чужой каталог здесь не листают. */
function UserItems({ userId }: { userId: string }) {
  const { data, isPending, isError } = useQuery({
    queryKey: itemKeys.ofUser(userId),
    queryFn: () => getUserItems(userId),
  })

  // Профиль уже показан: молчим о неудаче списка так же, как об отзывах — отдельной строкой.
  if (isPending || isError || data.length === 0) return null

  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-[19px] leading-6 font-bold">
        Вещи на обмене<span className="text-ink-2"> · {data.length}</span>
      </h2>

      <div className="flex flex-col">
        {data.map((item) => (
          <ItemCard key={item.id} item={item} />
        ))}
      </div>
    </section>
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
      <h2 className="text-[19px] leading-6 font-bold">
        Отзывы{count !== undefined && count > 0 && <span className="text-ink-2"> · {count}</span>}
      </h2>

      {data.length === 0 ? (
        <p className="text-[13px] text-ink-2">
          Отзывов пока нет — их оставляют соседи по кругу после завершённого обмена.
        </p>
      ) : (
        <TileGroup>
          {data.map((review) => (
            <div
              key={review.id}
              className="flex flex-col gap-1.5 px-4 py-3.5 not-first:border-t not-first:border-card"
            >
              <div className="flex items-center gap-2">
                <Avatar
                  name={review.author.name}
                  src={review.author.avatarUrl}
                  className="size-7"
                />
                {/* Имя ведёт в профиль автора: отзыв — довод о человеке, и первый вопрос
                    к нему «а кто это написал». */}
                <Link
                  to={`/users/${review.author.id}`}
                  className="rounded-sm text-[13px] font-bold outline-offset-2 hover:text-brand focus-visible:outline-2 focus-visible:outline-brand"
                >
                  {review.author.name}
                </Link>
                <span className="flex items-center gap-1 text-[13px] text-ink-2">
                  <Stars rating={review.rating} size={13} />
                </span>
                <span className="ml-auto text-[12.5px] text-ink-3">
                  {reviewDate(review.createdAt)}
                </span>
              </div>

              {/* Отзыв без текста — обычное дело: оценку ставят чаще, чем пишут. */}
              {review.text && <p className="text-[13px] leading-relaxed">{review.text}</p>}
            </div>
          ))}
        </TileGroup>
      )}
    </section>
  )
}
