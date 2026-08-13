import { currentPersonaId, editPersona, personaById } from '@/shared/model/persona'
import type { Profile, ProfileEdit, Report, Review } from '../model/types'

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
    // Завершённые обмены на моках выводим из отзывов: каждый оставляют после закрытой
    // сделки, поэтому число обменов не может быть меньше — и выдумывать его отдельно незачем.
    completedExchanges: persona.reviews,
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

/**
 * Отзывы на моках. Не выдумываем «отличный обмен» на каждого: тексты разной длины и
 * оценки не только пятёрки — иначе блок читается как витрина, а не как отзывы живых людей.
 * Один отзыв намеренно без текста: так и бывает, а интерфейс обязан это пережить.
 */
const REVIEWS: Record<string, Review[]> = {
  u1: [
    {
      id: 'r1',
      author: { id: 'u2', name: 'Марк', avatarUrl: '/mock/avatars/u12.jpg' },
      rating: 5,
      text: 'Вещь как в описании, в ПВЗ сдала в тот же день. Спасибо за обмен!',
      createdAt: '2026-08-04T12:20:00Z',
    },
    {
      id: 'r2',
      author: { id: 'u3', name: 'Лена', avatarUrl: '/mock/avatars/u47.jpg' },
      rating: 4,
      text: 'Всё хорошо, только ответа пришлось подождать пару дней.',
      createdAt: '2026-07-28T09:10:00Z',
    },
  ],
  u2: [
    {
      id: 'r3',
      author: { id: 'u1', name: 'Даша', avatarUrl: '/mock/avatars/u32.jpg' },
      rating: 5,
      createdAt: '2026-08-04T12:25:00Z',
    },
  ],
  u3: [],
}

export async function reviewsFor(id: string): Promise<Review[]> {
  await delay(250)
  return REVIEWS[id] ?? []
}

/**
 * Оставленный отзыв виден сразу, как и с бэкендом: демо на моках должно доигрываться
 * до конца, включая последний шаг обмена.
 */
export async function addReview(
  targetUserId: string,
  rating: number,
  text?: string,
): Promise<void> {
  await delay(400)
  const me = personaById(currentPersonaId())

  REVIEWS[targetUserId] = [
    {
      id: `r${Date.now()}`,
      author: { id: me?.id ?? 'u1', name: me?.name ?? 'Вы', avatarUrl: me?.avatarUrl || undefined },
      rating,
      text: text?.trim() || undefined,
      createdAt: new Date().toISOString(),
    },
    ...(REVIEWS[targetUserId] ?? []),
  ]
}

/**
 * Чёрный список и жалобы живут в памяти вкладки: ручек в контракте нет (версия 0.8.0),
 * а показать механизм защиты на защите надо — в кейсе он записан требованием.
 *
 * Обещать разбор жалобы нельзя, поэтому мок ровно такой, каким его видит человек:
 * отправили — приняли. Никакой очереди модерации за этим не стоит, и интерфейс её
 * не рисует.
 */
const BLOCKED = new Set<string>()
const REPORTS: Report[] = []

export async function blocked(): Promise<string[]> {
  await delay(150)
  return [...BLOCKED]
}

export async function block(userId: string): Promise<void> {
  await delay(300)
  BLOCKED.add(userId)
}

export async function unblock(userId: string): Promise<void> {
  await delay(300)
  BLOCKED.delete(userId)
}

export async function report(complaint: Report): Promise<void> {
  await delay(400)
  REPORTS.push(complaint)
}
