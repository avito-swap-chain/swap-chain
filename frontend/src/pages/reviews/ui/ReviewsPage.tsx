import { useState } from 'react'
import { useQueries, useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  chainKeys,
  findNeighbours,
  getMyChains,
  type Chain,
  type ChainParticipant,
} from '@/entities/chain'
import { getMyProfile, getReviews, userKeys } from '@/entities/user'
import { LeaveReview } from '@/features/leave-review'
import { asset, cx, formatDate, reviewsLabel } from '@/shared/lib'
import { Avatar, IconBox, Notice, Screen, Stars } from '@/shared/ui'

type Tab = 'pending' | 'about'

/**
 * «Мои отзывы» — отдельный раздел кабинета, как у Авито: там он тоже вынесен в меню
 * и открывается двумя вкладками. Первая — что ждёт оценки: завершённые обмены, где сосед
 * ещё не оценён. Вторая — что написали о самом человеке: в обмене рейтинг это единственный
 * довод «с кем я меняюсь», и прятать его в чужой профиль нельзя.
 *
 * Вкладки «Оставленные» у нас нет намеренно: контракт отдаёт отзывы о пользователе,
 * а не написанные им, и собирать её из ничего значило бы показать пустую страницу.
 */
export function ReviewsPage() {
  const [tab, setTab] = useState<Tab>('pending')

  const { data: me, isError: meFailed } = useQuery({
    queryKey: userKeys.me(),
    queryFn: getMyProfile,
  })

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">Мои отзывы</h1>

        {/* Вкладки набраны кеглем 21 полужирным, активная — чёрным с подчёркиванием:
            ровно как в их разделе отзывов. */}
        <div className="flex gap-4 border-b border-line">
          <Tabs tab={tab} onChange={setTab} />
        </div>

        {tab === 'pending' ? (
          <PendingList meId={me?.id} />
        ) : (
          <AboutList userId={me?.id} count={me?.reviews} failed={meFailed} />
        )}
      </div>
    </Screen>
  )
}

function Tabs({ tab, onChange }: { tab: Tab; onChange: (next: Tab) => void }) {
  const tabs: { key: Tab; label: string }[] = [
    { key: 'pending', label: 'Ждут оценки' },
    { key: 'about', label: 'Обо мне' },
  ]

  return (
    <>
      {tabs.map((item) => (
        <button
          key={item.key}
          type="button"
          aria-current={tab === item.key ? 'page' : undefined}
          onClick={() => onChange(item.key)}
          className={cx(
            'cursor-pointer border-b-[3px] pt-3 pb-2 text-[17px] leading-6 font-bold outline-offset-2 focus-visible:outline-2 focus-visible:outline-brand lg:text-[21px] lg:leading-[26px]',
            tab === item.key
              ? 'border-ink text-ink'
              : 'border-transparent text-ink-2 hover:text-ink',
          )}
        >
          {item.label}
        </button>
      ))}
    </>
  )
}

/**
 * Завершённые обмены. Оцениваем того, от кого получили вещь, — с ним человек и имел дело;
 * тот же сосед, что и на экране цепочки, поэтому оценка здесь тем же компонентом.
 */
function PendingList({ meId }: { meId?: string }) {
  const { data, isPending, isError } = useQuery({ queryKey: chainKeys.my(), queryFn: getMyChains })

  // Соседи по завершённым обменам — те, кого можно оценить. Считаем до ранних возвратов:
  // ниже на этом списке висят запросы, а хуки не переживают условный вызов.
  const pairs = (data ?? [])
    .filter((chain) => chain.status === 'completed')
    .map((chain) => ({ chain, giver: findNeighbours(chain)?.giver }))
    .filter((pair): pair is { chain: Chain; giver: ChainParticipant } => Boolean(pair.giver))

  /**
   * Оставлен ли отзыв, контракт не отдаёт: признака у цепочки нет, а второй отзыв бэкенд
   * отклоняет как 409. Поэтому спрашиваем отзывы самого соседа и ищем среди них свой —
   * иначе вкладка обещала бы оценку по обмену, который человек уже оценил.
   *
   * Запрос на соседа, но завершённых обменов у человека единицы, и ответы шарятся с профилем
   * по тому же ключу — лишней нагрузки это не создаёт.
   */
  const reviews = useQueries({
    queries: pairs.map((pair) => ({
      queryKey: userKeys.reviews(pair.giver.userId),
      queryFn: () => getReviews(pair.giver.userId),
      enabled: Boolean(meId),
    })),
  })

  if (isPending) return <Notice>Загрузка…</Notice>
  if (isError) return <Notice tone="error">Не удалось загрузить обмены</Notice>

  const waiting = pairs.filter((_, index) => {
    const given = reviews[index]?.data
    // Пока отзывы соседа не загрузились, строку показываем: пропасть она может только
    // по факту, а не по незнанию.
    return !given || !given.some((review) => review.author.id === meId)
  })

  if (waiting.length === 0) {
    return (
      <p className="py-4 text-[13px] text-ink-2">
        {pairs.length === 0
          ? 'Оценивать пока некого — отзыв оставляют соседу после завершённого обмена.'
          : 'Все завершённые обмены оценены. Спасибо: рейтинг — единственный довод о человеке, с которым меняются.'}
      </p>
    )
  }

  return (
    <ul className="flex flex-col">
      {waiting.map(({ chain, giver }) => {
        return (
          <li
            key={chain.id}
            className="flex flex-col gap-3 border-b border-line py-4 last:border-0 sm:flex-row sm:items-start sm:justify-between"
          >
            <Link to={`/exchange/${chain.id}`} className="flex items-center gap-4">
              {/* Слева — вещь, которую человек получил: по ней он и вспомнит, о каком
                  обмене речь. Имя соседа рядом: оценивают всё-таки человека. */}
              {giver.givesItem.photoUrl ? (
                <img
                  src={asset(giver.givesItem.photoUrl)}
                  alt=""
                  className="size-[60px] shrink-0 rounded-chip object-cover"
                  width={60}
                  height={60}
                />
              ) : (
                <span className="grid size-[60px] shrink-0 place-items-center rounded-chip bg-line-2 text-ink-3">
                  <IconBox size={24} />
                </span>
              )}
              <span className="flex flex-col gap-0.5">
                <span className="text-[15px] leading-5 font-bold">{giver.name}</span>
                <span className="text-[15px] leading-5 text-ink-2">{giver.givesItem.title}</span>
              </span>
            </Link>

            <div className="sm:w-[320px] sm:shrink-0">
              <LeaveReview chainId={chain.id} target={{ userId: giver.userId, name: giver.name }} />
            </div>
          </li>
        )
      })}
    </ul>
  )
}

/** Отзывы о самом человеке — тот же список, что видят на его профиле. */
function AboutList({
  userId,
  count,
  failed,
}: {
  userId?: string
  count?: number
  failed?: boolean
}) {
  const { data, isPending, isError } = useQuery({
    queryKey: userKeys.reviews(userId ?? ''),
    queryFn: () => getReviews(userId ?? ''),
    enabled: Boolean(userId),
  })

  // Профиль не загрузился — своих отзывов не найти, и «Загрузка…» здесь врала бы вечно.
  if (failed) return <Notice tone="error">Не удалось загрузить профиль</Notice>
  if (!userId || isPending) return <Notice>Загрузка…</Notice>
  if (isError) return <Notice tone="error">Не удалось загрузить отзывы</Notice>

  if (data.length === 0) {
    return (
      <p className="py-4 text-[13px] text-ink-2">
        Отзывов пока нет — их оставляют соседи по кругу после завершённого обмена.
      </p>
    )
  }

  return (
    <>
      {count !== undefined && count > 0 && (
        <p className="text-[13px] text-ink-2">{reviewsLabel(count)}</p>
      )}

      <ul className="flex flex-col">
        {data.map((review) => (
          <li
            key={review.id}
            className="flex flex-col gap-1.5 border-b border-line py-4 last:border-0"
          >
            <div className="flex items-center gap-2">
              <Avatar
                name={review.author.name}
                src={review.author.avatarUrl}
                className="size-8 text-[13px]"
              />
              <Link
                to={`/users/${review.author.id}`}
                className="text-[15px] leading-5 font-bold hover:text-brand"
              >
                {review.author.name}
              </Link>
              <Stars rating={review.rating} size={16} />
              <span className="ml-auto text-[13px] leading-4 text-ink-3">
                {formatDate(review.createdAt)}
              </span>
            </div>
            {review.text && <p className="text-[15px] leading-[22px]">{review.text}</p>}
          </li>
        ))}
      </ul>
    </>
  )
}
