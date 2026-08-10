import { ApiError, unwrap } from '@/shared/api/fetcher'
import {
  createUser,
  getSession,
  login as loginRequest,
  logout as logoutRequest,
} from '@/shared/api/generated/endpoints'
import type { Session } from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import { currentPersonaId, PERSONAS } from './persona'

/**
 * Кто сейчас в кабинете. На моках это выбранная персона, с бэкендом — владелец сессии.
 * Живёт в shared по той же причине, что и реестр персон: «кто я» читают и api в entities,
 * и переключатель с экраном входа в features, а entities о features знать не могут.
 *
 * Рейтинг и отзывы необязательны: у бэкенда их нет, и рисовать чужие звёзды поверх
 * реального пользователя нельзя — рейтинг в обмене это довод, а не украшение.
 */
export interface CurrentUser {
  id: string
  name: string
  /**
   * Телефон — единственный устойчивый ключ пользователя, который есть и в сессии, и в списке
   * демо-персон. Имена сравнивать нельзя: они повторяются и меняются, а переключатель на них
   * промахивался. На моках телефона нет — там персону опознаёт `id`.
   */
  phone?: string
  avatarUrl?: string
  rating?: number
  reviews?: number
  /**
   * Роль есть в контракте админки ПВЗ (прислан отдельно, в общий `openapi.yaml` ещё не влит —
   * номер версии там совпал с нашей, но набор ручек другой). В том, что раскатано, роли нет,
   * поэтому поле необязательное: интерфейс просто не помечает аккаунт, пока она не придёт.
   */
  isAdmin?: boolean
}

export const sessionKeys = { current: () => ['session'] as const }

/**
 * Демо-пользователи, засеянные бэкендом при старте. Вход только по телефону: пароля
 * в контракте нет, сессия выдаётся в обмен на номер.
 *
 * Роль здесь наша, из этой же таблицы: бэкенд говорит только про текущую сессию, а список
 * надо помечать до входа — иначе на демо не видно, под кем открывать админку.
 */
export const DEMO_USERS: { name: string; phone: string; isAdmin?: boolean }[] = [
  { name: 'Алиса', phone: '+79001000001' },
  { name: 'Борис', phone: '+79001000002' },
  { name: 'Вера', phone: '+79001000003' },
  // Сотрудник ПВЗ: проводит вещи по статусам доставки. Без него цепочка не закрывается —
  // отметить получение можно только после того, как вещь ушла в доставку.
  // Подпись повторяет имя из миграции, иначе в переключателе и в кабинете разные люди.
  { name: 'ПВЗ Администратор', phone: '+79009999999', isAdmin: true },
]

/**
 * Идентификатор владельца сессии. Держим в модуле: сессия одна на приложение, а
 * запрашивать её перед каждым чтением цепочек — лишний round-trip на каждый рендер.
 * Сбрасывается при входе и выходе, иначе после смены пользователя «я» останется прежним.
 */
let sessionUserId: number | undefined

const fromSession = (session: Session): CurrentUser => {
  sessionUserId = session.user.id

  // `role` живёт в контракте админки, которого нет в общем `openapi.yaml`, — читаем мягко,
  // чтобы не ждать его вливания: до тех пор поле просто не приходит.
  const { role } = session.user as typeof session.user & { role?: string }

  return {
    id: String(session.user.id),
    name: session.user.username,
    phone: session.user.phone,
    isAdmin: role === 'ADMIN',
  }
}

const fromPersona = (): CurrentUser => {
  const persona = PERSONAS.find((item) => item.id === currentPersonaId()) ?? PERSONAS[0]

  return {
    id: persona.id,
    name: persona.name,
    avatarUrl: persona.avatarUrl,
    rating: persona.rating,
    reviews: persona.reviews,
  }
}

/** Текущий пользователь или `null`, если сессии нет и нужно войти. */
export async function getCurrentUser(): Promise<CurrentUser | null> {
  if (!isBackendConnected) return fromPersona()

  try {
    return fromSession(unwrap<Session>(await getSession()))
  } catch (error) {
    // 401 — это не сбой, а «ещё не вошли». Экрану входа нужен ответ, а не исключение.
    if (error instanceof ApiError && error.status === 401) {
      sessionUserId = undefined
      return null
    }
    throw error
  }
}

/** Кто «я» для api, которым нужно отличить свои данные от чужих. */
export async function currentUserId(): Promise<number> {
  if (sessionUserId === undefined) {
    sessionUserId = unwrap<Session>(await getSession()).user.id
  }
  return sessionUserId
}

export async function login(phone: string): Promise<CurrentUser> {
  return fromSession(unwrap<Session>(await loginRequest({ phone })))
}

export async function register(username: string, phone: string): Promise<CurrentUser> {
  return fromSession(unwrap<Session>(await createUser({ username, phone })))
}

export async function logout(): Promise<void> {
  unwrap(await logoutRequest())
  sessionUserId = undefined
}
