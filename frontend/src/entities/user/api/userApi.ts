import { unwrap } from '@/shared/api/fetcher'
import {
  blockUser as blockUserRequest,
  createChainReview,
  getUser,
  listBlocks,
  listUserReviews,
  unblockUser as unblockUserRequest,
  updateUser,
  uploadMedia,
} from '@/shared/api/generated/endpoints'
import type {
  Block,
  BlockList,
  MediaUpload,
  UserProfile,
  UserReview,
  UserReviewList,
} from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import { currentUserId } from '@/shared/model/session'
import * as mock from './userMocks'
import type { BlockedUser, Profile, ProfileEdit, Report, Review } from '../model/types'

export const userKeys = {
  all: ['users'] as const,
  profile: (id: string) => [...userKeys.all, id] as const,
  me: () => [...userKeys.all, 'me'] as const,
  reviews: (id: string) => [...userKeys.all, id, 'reviews'] as const,
  blocked: () => [...userKeys.all, 'blocked'] as const,
}

const mapProfile = (profile: UserProfile): Profile => ({
  id: String(profile.id),
  name: profile.username,
  // Бэкенд отдаёт `null` у пользователя без фотографии, а интерфейсу нужна её нехватка.
  avatarUrl: profile.avatarUrl ?? undefined,
  registeredAt: profile.createdAt,
  // `null` в рейтинге — «его ещё никто не оценивал», а не ноль: у нуля на шкале от единицы
  // до пятёрки нет смысла, и показывать его нельзя.
  rating: profile.rating ?? undefined,
  reviews: profile.reviewsCount,
  completedExchanges: profile.completedExchanges,
})

const mapReview = (review: UserReview): Review => ({
  id: String(review.id),
  // Фотографии автора в отзыве контракт не отдаёт — `UserSummary` это только id и имя.
  // Аватар не подставляем: отзыв ведёт в профиль, там она и есть.
  author: { id: String(review.author.id), name: review.author.username },
  rating: review.rating,
  text: review.text ?? undefined,
  createdAt: review.createdAt,
})

/**
 * Отзывы о человеке. Читаются публично — это и есть довод «с кем я меняюсь»,
 * который до 0.8.0 заменялся датой регистрации.
 */
export async function getReviews(id: string): Promise<Review[]> {
  if (!isBackendConnected) return mock.reviewsFor(id)

  const { reviews } = unwrap<UserReviewList>(await listUserReviews(Number(id)))
  return reviews.map(mapReview)
}

/**
 * Отзыв соседу по завершённому обмену. Бэкенд пускает только участника цепочки в статусе
 * `COMPLETED` и только на прямого соседа по кругу — второй отзыв тому же человеку по той же
 * цепочке отклоняется 409, поэтому кнопка после отправки исчезает.
 */
export async function leaveReview(
  chainId: string,
  { targetUserId, rating, text }: { targetUserId: string; rating: number; text?: string },
): Promise<void> {
  if (!isBackendConnected) return mock.addReview(targetUserId, rating, text)

  await createChainReview(Number(chainId), {
    targetUserId: Number(targetUserId),
    rating,
    ...(text?.trim() ? { text: text.trim() } : {}),
  })
}

/** Профиль любого пользователя: ручка публичная, за ней и открывают соседа по цепочке. */
export async function getProfile(id: string): Promise<Profile> {
  if (!isBackendConnected) return mock.profileById(id)

  return mapProfile(unwrap<UserProfile>(await getUser(Number(id))))
}

/** Свой профиль. Идентификатор берём из сессии — в адресе его нет и быть не должно. */
export async function getMyProfile(): Promise<Profile> {
  if (!isBackendConnected) return mock.myProfile()

  return getProfile(String(await currentUserId()))
}

/**
 * Правка своего профиля. Пустые поля не отправляем: у имени `minLength: 1`, а пустая строка
 * в `avatarUrl` — осмысленное значение, «убрать фотографию», и путать эти два случая нельзя.
 */
export async function editProfile(id: string, patch: ProfileEdit): Promise<Profile> {
  const name = patch.name?.trim()
  const changes: ProfileEdit = {
    ...(name ? { name } : {}),
    ...(patch.avatarUrl === undefined ? {} : { avatarUrl: patch.avatarUrl }),
  }

  if (!isBackendConnected) return mock.edit(id, changes)

  return mapProfile(
    unwrap<UserProfile>(
      await updateUser(Number(id), {
        ...(changes.name ? { username: changes.name } : {}),
        ...(changes.avatarUrl === undefined ? {} : { avatarUrl: changes.avatarUrl }),
      }),
    ),
  )
}

/**
 * Загрузить фотографию профиля. Бэкенд принимает только свои же адреса из `POST /media`,
 * поэтому файл сначала уезжает в хранилище, а в профиль идёт выданная ссылка.
 *
 * На моках хранилища нет — там достаточно `blob:`-ссылки: она живёт столько же, сколько
 * открытая вкладка, а демо и есть одна вкладка.
 */
export async function uploadAvatar(file: File): Promise<string> {
  if (!isBackendConnected) return URL.createObjectURL(file)

  return unwrap<MediaUpload>(await uploadMedia({ file })).url
}

/**
 * Чёрный список. С контракта 0.9.0 у него есть ручки, и блокировка перестала быть
 * пометкой в браузере: бэкенд убирает пару из подбора в обе стороны и распускает общие
 * незавершённые цепочки. Интерфейс остался прежним — поменялось тело.
 *
 * Список читаем одной страницей: чёрный список — это единицы людей, а не лента, и
 * листать там нечего. Сотня записей — потолок, который разрешает контракт.
 */
/**
 * Кого человек заблокировал. Возвращаем имена, а не только идентификаторы: тот же список
 * показывает раздел «Чёрный список» в кабинете, а имён у него взять больше неоткуда —
 * профили заблокированных он не грузит.
 */
export async function getBlocked(): Promise<BlockedUser[]> {
  if (!isBackendConnected) return mock.blocked()

  const { blocks } = unwrap<BlockList>(await listBlocks({ limit: 100 }))
  return blocks.map((block) => ({
    id: String(block.blockedUser.id),
    name: block.blockedUser.username,
  }))
}

export async function blockUser(userId: string): Promise<void> {
  if (!isBackendConnected) return mock.block(userId)

  unwrap<Block>(await blockUserRequest({ blockedUserId: Number(userId) }))
}

export async function unblockUser(userId: string): Promise<void> {
  if (!isBackendConnected) return mock.unblock(userId)

  await unblockUserRequest(Number(userId))
}

/**
 * Жалобы. Ручка в контракте есть, но она про сообщение чата, а не про человека —
 * см. `entities/chat`. Здесь остаётся мок ровно для того же, для чего он был:
 * пожаловаться на пользователя из его профиля, где никакого сообщения нет.
 */
export const reportUser = (complaint: Report): Promise<void> => mock.report(complaint)
