import { notify } from '@/shared/model/notifications'
import { currentPersonaId } from '@/shared/model/persona'
import type { Message, Thread, ThreadRef } from '../model/types'

/**
 * Ключи кэша TanStack Query. `list` — префикс `thread`, поэтому инвалидация списка
 * обновляет и открытый диалог.
 */
export const messageKeys = {
  all: ['messages'] as const,
  list: () => messageKeys.all,
  thread: (itemId: string) => [...messageKeys.all, itemId] as const,
  /** Счётчик в шапке: отдельный ключ, но под общим префиксом — инвалидируется вместе со списком. */
  unread: () => [...messageKeys.all, 'unread'] as const,
}

/**
 * Заготовки вопросов о состоянии. Это не украшение: соразмерность в цепочке обеспечена тем,
 * что каждый получает названную им вещь, — но устроит ли конкретный экземпляр, видно только
 * из разговора с владельцем, и спросить надо до согласия.
 */
export const QUICK_QUESTIONS = [
  'В каком состоянии вещь?',
  'Есть дефекты или следы использования?',
  'Комплект полный?',
  'Давно пользуетесь?',
] as const

/**
 * Ответы владельца подобраны по смыслу вопроса — так виден сам сценарий «уточнить состояние»,
 * а не просто эхо. В бою здесь живой собеседник, поэтому логика намеренно тривиальная.
 */
const REPLIES: [RegExp, (title: string) => string][] = [
  [
    /дефект|царап|скол|потёрт|потерт|след|состоян|битый|рабоч/i,
    () =>
      'Состояние хорошее: следов почти нет, есть небольшая царапина на корпусе снизу — на работу не влияет. Могу прислать фото этого места.',
  ],
  [
    /комплект|коробк|документ|чек|зарядк|провод/i,
    () => 'Комплект полный: коробка, зарядка и документы на месте.',
  ],
  [/давно|сколько|лет|год|польз|купил/i, () => 'Пользуюсь полтора года, хранил дома, не ронял.'],
  [
    /фото|снимок|видео|покаж/i,
    (title) => `Сфотографирую ${title.toLowerCase()} при дневном свете и пришлю сегодня.`,
  ],
  [
    /встрет|обмен|когда|где|переда|привез|привёз/i,
    () => 'По передаче договоримся, когда цепочка соберётся — сервис подскажет очередь.',
  ],
]

const FALLBACK = 'Отвечу в течение дня. Если что-то важно уточнить до обмена — спрашивайте.'

const replyTo = (text: string, itemTitle: string): string => {
  const matched = REPLIES.find(([pattern]) => pattern.test(text))
  return matched ? matched[1](itemTitle) : FALLBACK
}

let counter = 0
const nextId = (): string => `m${++counter}`

const message = (author: Message['author'], text: string, createdAt = new Date()): Message => ({
  id: nextId(),
  author,
  text,
  createdAt: createdAt.toISOString(),
})

/**
 * Служебный тред сервиса — он же первый в списке. У Авито мессенджер никогда не бывает
 * совсем пустым: сверху всегда стоит канал самого Авито. Здесь он объясняет правило,
 * которое иначе пришлось бы объяснять в интерфейсе цепочки.
 */
const serviceThread = (): Thread => ({
  itemId: 'service',
  itemTitle: 'Обмен без доплат',
  peerName: 'Авито Обмен',
  unread: true,
  messages: [
    message(
      'system',
      'В обмене нет цен и доплат: вы получаете ровно то, что указали в желании. Прежде чем согласиться, уточните у владельца состояние вещи — по фото и описанию видно не всё.',
      new Date(Date.now() - 36e5),
    ),
  ],
})

// Мок вместо бэкенда. Треды раскладываются по персонам: переключение персоны в демо
// меняет и точку зрения на переписку.
let threadsByPersona: Record<string, Thread[]> = {}

const threadsOf = (personaId: string): Thread[] => threadsByPersona[personaId] ?? [serviceThread()]

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

/** Время последней реплики — по нему список сортируется, как в мессенджере. */
const lastAt = (thread: Thread): number => {
  const last = thread.messages.at(-1)
  return last ? Date.parse(last.createdAt) : 0
}

/** Все переписки текущей персоны, свежие сверху. */
export async function getThreads(): Promise<Thread[]> {
  await delay(250)
  return [...threadsOf(currentPersonaId())].sort((a, b) => lastAt(b) - lastAt(a))
}

/** Одна переписка. `undefined` — разговор ещё не начинали. */
export async function getThread(itemId: string): Promise<Thread | undefined> {
  await delay(200)
  return threadsOf(currentPersonaId()).find((thread) => thread.itemId === itemId)
}

/**
 * Отправка сообщения. Ответ владельца приходит сразу же: в демо важно показать сценарий
 * целиком, а не ожидание — задержка ответа ничего не объясняет, но ломает показ.
 * Первое сообщение заводит тред, поэтому подпись собеседника передаётся вместе с текстом.
 */
export async function sendMessage({ text, ...ref }: ThreadRef & { text: string }): Promise<Thread> {
  await delay(300)

  const personaId = currentPersonaId()
  const threads = threadsOf(personaId)
  const existing = threads.find((thread) => thread.itemId === ref.itemId)

  const updated: Thread = {
    ...ref,
    // Ответ собеседника приходит сразу же, поэтому переписка становится непрочитанной —
    // и гаснет, как только человек её откроет.
    unread: true,
    messages: [
      ...(existing?.messages ?? []),
      message('me', text),
      message('them', replyTo(text, ref.itemTitle)),
    ],
  }

  threadsByPersona = {
    ...threadsByPersona,
    [personaId]: [...threads.filter((thread) => thread.itemId !== ref.itemId), updated],
  }

  notify({
    kind: 'message',
    title: `Новое сообщение от ${ref.peerName}`,
    text: updated.messages.at(-1)?.text ?? '',
    to: `/messages/${ref.itemId}`,
  })

  return updated
}

/** Сколько переписок ждут прочтения — счётчик на иконке сообщений в шапке. */
export async function countUnread(): Promise<number> {
  await delay(150)
  return threadsOf(currentPersonaId()).filter((thread) => thread.unread).length
}

/** Переписку открыли — непрочитанное снимается. */
export async function markThreadRead(itemId: string): Promise<void> {
  const personaId = currentPersonaId()
  const threads = threadsOf(personaId)
  if (!threads.some((thread) => thread.itemId === itemId && thread.unread)) return

  threadsByPersona = {
    ...threadsByPersona,
    [personaId]: threads.map((thread) =>
      thread.itemId === itemId ? { ...thread, unread: false } : thread,
    ),
  }
}

/** Сброс переписок — для тестов. */
export const resetThreads = (): void => {
  threadsByPersona = {}
}
