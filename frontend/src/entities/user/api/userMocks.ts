import { currentPersonaId, editPersona, personaById } from '@/shared/model/persona'
import type { Profile, ProfileEdit } from '../model/types'

/**
 * Профили на моках — тот же реестр демо-персон, из которого собираются участники цепочек
 * и кабинет. Правки уходят туда же (`editPersona`), иначе человек поменял бы имя в профиле
 * и не увидел его ни в шапке, ни в цепочке: один человек выглядел бы по-разному.
 */

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

/** Дата регистрации у персон выдуманная, но одинаковая от запуска к запуску. */
const REGISTERED_AT = '2026-03-14T09:00:00Z'

function toProfile(id: string): Profile {
  const persona = personaById(id)
  if (!persona) throw new Error(`Профиль ${id} не найден`)

  return {
    id: persona.id,
    name: persona.name,
    avatarUrl: persona.avatarUrl || undefined,
    registeredAt: REGISTERED_AT,
    rating: persona.rating,
    reviews: persona.reviews,
  }
}

export async function profileById(id: string): Promise<Profile> {
  await delay(250)
  return toProfile(id)
}

export async function myProfile(): Promise<Profile> {
  return profileById(currentPersonaId())
}

export async function edit(id: string, patch: ProfileEdit): Promise<Profile> {
  await delay(400)

  editPersona(id, { name: patch.name, avatarUrl: patch.avatarUrl })
  return toProfile(id)
}
