import type { ThreadKey } from '../model/types'

/**
 * Адрес служебного канала сервиса. Пара нечисловых идентификаторов выбрана намеренно:
 * у настоящих переписок это номер вещи и номер пользователя, и спутать их нельзя.
 */
export const SERVICE_THREAD: ThreadKey = { itemId: 'support', counterpartId: 'support' }

/**
 * Переписка с поддержкой. Адрес служебный: вещи у этого разговора нет, а место в списке
 * — всегда первое. С контракта 0.11.0 за ним стоит настоящий тред на бэкенде, до этого
 * канал был фронтовым пояснением.
 */
export const isServiceThread = (key: ThreadKey): boolean => key.itemId === SERVICE_THREAD.itemId

export const sameThread = (a: ThreadKey, b: ThreadKey): boolean =>
  a.itemId === b.itemId && a.counterpartId === b.counterpartId

/** Адрес экрана переписки. Собран в одном месте: на него ссылаются список, карточка и уведомления. */
export const threadPath = (key: ThreadKey): string => `/messages/${key.itemId}/${key.counterpartId}`

/**
 * Порядок списка переписок: служебный канал сервиса всегда первый, остальные — как отдал
 * бэкенд (по свежести последней реплики). Правило одно на раздел и на плавающий мессенджер:
 * два списка одних и тех же разговоров не должны расходиться порядком.
 */
export const orderThreads = <T extends ThreadKey>(threads: T[]): T[] =>
  [...threads].sort((a, b) => Number(isServiceThread(b)) - Number(isServiceThread(a)))
