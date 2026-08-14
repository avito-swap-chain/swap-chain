import type { ThreadKey } from '../model/types'

/**
 * Адрес служебного канала сервиса. Пара нечисловых идентификаторов выбрана намеренно:
 * у настоящих переписок это номер вещи и номер пользователя, и спутать их нельзя.
 */
export const SERVICE_THREAD: ThreadKey = { itemId: 'service', counterpartId: 'service' }

export const isServiceThread = (key: ThreadKey): boolean => key.itemId === SERVICE_THREAD.itemId

export const sameThread = (a: ThreadKey, b: ThreadKey): boolean =>
  a.itemId === b.itemId && a.counterpartId === b.counterpartId

/** Адрес экрана переписки. Собран в одном месте: на него ссылаются список, карточка и уведомления. */
export const threadPath = (key: ThreadKey): string => `/messages/${key.itemId}/${key.counterpartId}`
