import type { ThreadKey } from '../model/types'

/**
 * Адрес служебного канала сервиса. Пара нечисловых идентификаторов выбрана намеренно:
 * у настоящих переписок это номер цепочки и номер пользователя, и спутать их нельзя.
 */
export const SERVICE_THREAD: ThreadKey = { chainId: 'service', counterpartId: 'service' }

export const isServiceThread = (key: ThreadKey): boolean => key.chainId === SERVICE_THREAD.chainId

export const sameThread = (a: ThreadKey, b: ThreadKey): boolean =>
  a.chainId === b.chainId && a.counterpartId === b.counterpartId

/** Адрес экрана переписки. Собран в одном месте: на него ссылаются список, карточка и уведомления. */
export const threadPath = (key: ThreadKey): string =>
  `/messages/${key.chainId}/${key.counterpartId}`
