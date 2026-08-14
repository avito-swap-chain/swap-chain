/**
 * Кто написал реплику. Роль, а не идентификатор пользователя: диалог всегда один на один
 * с соседом по кругу обмена, и на экране важно только «моё сообщение или его». Контракт
 * присылает отправителя целиком, роль считается в маппере сравнением с владельцем сессии.
 *
 * `system` с бэкенда прийти не может: в контракте отправитель — всегда пользователь.
 * Эта роль осталась ради служебного канала сервиса, который живёт только на фронте.
 */
export type MessageAuthor = 'me' | 'them' | 'system'

export interface Message {
  id: string
  author: MessageAuthor
  text: string
  /** ISO-время отправки; в ленте показываем только часы и минуты. */
  createdAt: string
}

/**
 * Адрес переписки: вещь и собеседник. Тема разговора — вещь, которая переезжает от одного
 * к другому, а не цепочка: одна и та же вещь у того же человека может участвовать в разных
 * вариантах обмена, и разрывать историю по ним значит терять уже сказанное. Так решил
 * бэкенд в контракте 0.10.0 — до этого адресом была пара «цепочка + собеседник».
 */
export interface ThreadKey {
  itemId: string
  counterpartId: string
}

/**
 * Подписи переписки: с кем и о чём. Вещь у настоящего треда есть всегда — она и есть его
 * адрес; необязательным поле осталось ради служебного канала сервиса, у которого темы нет.
 */
export interface ThreadRef extends ThreadKey {
  peerName: string
  peerAvatarUrl?: string
  itemTitle?: string
  itemPhotoUrl?: string
}

/**
 * Строка списка переписок. Реплик здесь нет: контракт отдаёт их отдельной ручкой с курсором,
 * а списку хватает последней — ровно её и показывает мессенджер.
 */
export interface Thread extends ThreadRef {
  lastMessage?: Message
  /** Сколько чужих реплик пришло после моей отметки о прочтении. */
  unreadCount: number
}

/**
 * Список переписок вместе с общим счётчиком. Счётчик приходит тем же ответом, потому что
 * отдельной ручки для него в контракте нет — так задумано: шапка и список обновляются разом.
 */
export interface ThreadList {
  threads: Thread[]
  totalUnread: number
}

/** Что передаётся в отправку: текст и ключ идемпотентности, выданный на само действие. */
export interface MessageDraft {
  text: string
  clientMessageId: string
}

/**
 * За что можно пожаловаться на реплику. Список короткий и берётся из контракта: жалоба
 * уезжает в очередь модерации, и своих причин туда не добавить — разбирающий их не увидит.
 */
export type MessageReportReason = 'spam' | 'abuse' | 'other'

export const MESSAGE_REPORT_REASON_LABEL: Record<MessageReportReason, string> = {
  spam: 'Спам или реклама',
  abuse: 'Оскорбления и грубость',
  other: 'Другое',
}

export const MESSAGE_REPORT_REASONS = Object.keys(
  MESSAGE_REPORT_REASON_LABEL,
) as MessageReportReason[]

/** Параметры чтения ленты: курсор и сколько секунд бэкенду держать запрос, ожидая нового. */
export interface ReadOptions {
  afterId?: string
  waitSeconds?: number
  signal?: AbortSignal
}
