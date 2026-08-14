import { ApiError, unwrap } from '@/shared/api/fetcher'
import {
  createUser,
  getSession,
  login as loginRequest,
  logout as logoutRequest,
} from '@/shared/api/generated/endpoints'
import type { Session } from '@/shared/api/generated/model'
import { formatPhone } from '@/shared/lib'
import { isBackendConnected } from '@/shared/config/backend'
import { currentPersonaId, personaById, PERSONAS } from './persona'

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
   * Сотрудник ПВЗ: у него открыто рабочее место с доставками. Роль приходит с сессией
   * (`CurrentUser.role` в контракте), но на моках сессии нет — там «я» это выбранная персона,
   * и роли взять неоткуда. Поэтому поле необязательное.
   */
  isAdmin?: boolean
}

export const sessionKeys = { current: () => ['session'] as const }

/**
 * Демо-номера, засеянные бэкендом при старте. Вход только по телефону: пароля в контракте
 * нет, сессия выдаётся в обмен на номер.
 *
 * Имён здесь намеренно нет. Раньше список был подписан «Алиса», «Борис», «Вера» — а базу
 * пересеяли, и те же номера стали принадлежать другим людям: кнопка обещала Бориса и пускала
 * Даниила. Кто стоит за номером, знает только бэкенд, и узнаётся это входом — поэтому до
 * первого входа показываем номер, а после подставляем настоящее имя из `knownAccounts`.
 *
 * Роль — исключение: она задана миграцией, а не сидом, и её надо знать до входа, иначе
 * на демо не понять, под кем открывать рабочее место ПВЗ.
 */
export const DEMO_USERS: { phone: string; isAdmin?: boolean }[] = [
  { phone: '+79001000001' },
  { phone: '+79001000002' },
  { phone: '+79001000003' },
  // Сотрудник ПВЗ: проводит вещи по статусам доставки. Без него цепочка не закрывается —
  // отметить получение можно только после того, как вещь ушла в доставку.
  { phone: '+79009999999', isAdmin: true },
]

/**
 * Как подписать демо-номер: настоящим именем, если под ним уже входили с этого браузера,
 * иначе самим номером. Выдумывать имя нельзя — оно и было причиной расхождения с базой.
 */
export function accountLabel(phone: string, isAdmin?: boolean): string {
  const known = knownAccounts().find((account) => account.phone === phone)
  if (known) return known.name

  return isAdmin ? `${formatPhone(phone)} · сотрудник ПВЗ` : formatPhone(phone)
}

/** Аккаунт, под которым уже входили с этого браузера: то же, из чего состоит `DEMO_USERS`. */
export interface KnownAccount {
  name: string
  phone: string
  isAdmin?: boolean
}

/**
 * Демо-пользователей засевает бэкенд, а тот, кто зарегистрировался своим номером, не значится
 * нигде — и после переключения на другого человека собственный аккаунт пропадал из списка:
 * вернуться в него можно было, только набрав номер заново. Поэтому входы запоминаем.
 *
 * Храним в браузере, а не в памяти модуля: перезагрузка страницы на демо — обычное дело,
 * и список обнулялся бы ровно с тем же результатом.
 */
const KNOWN_ACCOUNTS_KEY = 'barter.known-accounts'

export function knownAccounts(): KnownAccount[] {
  try {
    const stored: unknown = JSON.parse(localStorage.getItem(KNOWN_ACCOUNTS_KEY) ?? '[]')
    return Array.isArray(stored) ? stored : []
  } catch {
    // Мусор в хранилище (например, формат прошлой версии) — не повод ронять шапку.
    return []
  }
}

/** Список пополняется сам при каждом чтении сессии: отдельного действия для этого нет. */
function rememberAccount(user: CurrentUser): void {
  if (!user.phone) return

  const others = knownAccounts().filter((account) => account.phone !== user.phone)
  const account: KnownAccount = { name: user.name, phone: user.phone, isAdmin: user.isAdmin }

  localStorage.setItem(KNOWN_ACCOUNTS_KEY, JSON.stringify([account, ...others]))
}

/**
 * Идентификатор владельца сессии. Держим в модуле: сессия одна на приложение, а
 * запрашивать её перед каждым чтением цепочек — лишний round-trip на каждый рендер.
 * Сбрасывается при входе и выходе, иначе после смены пользователя «я» останется прежним.
 */
let sessionUserId: number | undefined

const fromSession = (session: Session): CurrentUser => {
  sessionUserId = session.user.id

  const user: CurrentUser = {
    id: String(session.user.id),
    name: session.user.username,
    phone: session.user.phone,
    isAdmin: session.user.role === 'ADMIN',
  }

  rememberAccount(user)
  return user
}

const fromPersona = (): CurrentUser => {
  const persona = personaById(currentPersonaId()) ?? PERSONAS[0]

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
