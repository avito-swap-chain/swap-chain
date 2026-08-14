import type { Message, Thread } from '../model/types'

/**
 * Поддержка на моках. С бэкендом это настоящий тред с модератором (`supportApi`), а здесь —
 * то же место в списке, занятое пояснением от сервиса: демо обязано открываться без сервера,
 * а разговаривать в нём не с кем.
 *
 * У Авито мессенджер никогда не бывает совсем пустым: сверху всегда стоит канал самого Авито.
 */
const SERVICE_MESSAGE: Message = {
  id: 'service',
  author: 'system',
  text: 'В обмене нет цен и доплат: вы получаете ровно то, что указали в желании. Прежде чем согласиться, уточните у владельца состояние вещи — по фото и описанию видно не всё.',
  createdAt: new Date(Date.now() - 36e5).toISOString(),
}

/**
 * Отметка о прочтении. Живёт в `localStorage`, а не в памяти вкладки: канала нет на бэкенде,
 * а в памяти он «забывал» прочтение на каждой перезагрузке — человек читал пояснение, обновлял
 * страницу и снова видел непрочитанное сообщение и счётчик в шапке.
 *
 * Ключ — на владельца сессии: демо переключает персон, и общая отметка гасила бы канал
 * у всех сразу, стоило одному из них его открыть.
 */
const readKey = (ownerId: string) => `avito-chain:service-thread-read:${ownerId}`

/** Хранилища может не быть — приватный режим, запрет, тесты вне браузера. */
const storage = (): Storage | undefined => {
  try {
    return globalThis.localStorage
  } catch {
    return undefined
  }
}

/** Прочитанное в этой вкладке: хранилище может быть недоступно, а счётчик гаснуть должен. */
const readInMemory = new Set<string>()

const isRead = (ownerId: string): boolean =>
  readInMemory.has(ownerId) || storage()?.getItem(readKey(ownerId)) === '1'

export const markServiceRead = (ownerId: string): void => {
  readInMemory.add(ownerId)
  storage()?.setItem(readKey(ownerId), '1')
}

export const serviceMessages = (): Message[] => [SERVICE_MESSAGE]

export const serviceThread = (ownerId: string): Thread => ({
  itemId: 'support',
  counterpartId: 'support',
  peerName: 'Поддержка Авито',
  itemTitle: 'Обмен без доплат',
  lastMessage: SERVICE_MESSAGE,
  unreadCount: isRead(ownerId) ? 0 : 1,
})

/** Сброс для тестов: канал общий на приложение, иначе прочитанность течёт между случаями. */
export const resetServiceThread = (): void => {
  for (const ownerId of readInMemory) storage()?.removeItem(readKey(ownerId))
  readInMemory.clear()
}
