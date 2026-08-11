import { unwrap } from '@/shared/api/fetcher'
import { getUser, updateUser, uploadMedia } from '@/shared/api/generated/endpoints'
import type { MediaUpload, UserProfile } from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import { currentUserId } from '@/shared/model/session'
import * as mock from './userMocks'
import type { Profile, ProfileEdit } from '../model/types'

export const userKeys = {
  all: ['users'] as const,
  profile: (id: string) => [...userKeys.all, id] as const,
  me: () => [...userKeys.all, 'me'] as const,
}

const mapProfile = (profile: UserProfile): Profile => ({
  id: String(profile.id),
  name: profile.username,
  // Бэкенд отдаёт `null` у пользователя без фотографии, а интерфейсу нужна её нехватка.
  avatarUrl: profile.avatarUrl ?? undefined,
  registeredAt: profile.createdAt,
})

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
