import type { Message, Thread } from '../model/types'

/**
 * Служебный канал сервиса — он же первый в списке переписок. У Авито мессенджер никогда
 * не бывает совсем пустым: сверху всегда стоит канал самого Авито. Здесь он объясняет правило,
 * которое иначе пришлось бы объяснять в интерфейсе цепочки.
 *
 * Это не мок, а часть продукта: канал живёт и с подключённым бэкендом. В контракте его нет
 * и быть не может — там отправитель сообщения всегда пользователь, а это пояснение интерфейса,
 * а не разговор. Согласовано с бэкендом именно так.
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
 */
const READ_KEY = 'avito-chain:service-thread-read'

/** Хранилища может не быть — приватный режим, запрет, тесты вне браузера. */
const storage = (): Storage | undefined => {
  try {
    return globalThis.localStorage
  } catch {
    return undefined
  }
}

let isRead = storage()?.getItem(READ_KEY) === '1'

export const markServiceRead = (): void => {
  isRead = true
  storage()?.setItem(READ_KEY, '1')
}

export const serviceMessages = (): Message[] => [SERVICE_MESSAGE]

export const serviceThread = (): Thread => ({
  itemId: 'service',
  counterpartId: 'service',
  peerName: 'Авито Обмен',
  itemTitle: 'Обмен без доплат',
  lastMessage: SERVICE_MESSAGE,
  unreadCount: isRead ? 0 : 1,
})

/** Сброс для тестов: канал общий на приложение, иначе прочитанность течёт между случаями. */
export const resetServiceThread = (): void => {
  isRead = false
  storage()?.removeItem(READ_KEY)
}
