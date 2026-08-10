import type { AdminDeliveryTransitionStatus } from '@/shared/api/generated/model'
import type { DeliveryStatus } from '../model/types'

/**
 * Куда доставку можно перевести. В `AWAITING_PVZ` вещь не возвращается: приёмка на стойке —
 * действие физическое, и отменить его отметкой в интерфейсе нельзя.
 */
export type DeliveryTransition = AdminDeliveryTransitionStatus

/**
 * Следующий шаг пути. Таблицей, а не сдвигом по массиву: `Record` требует ответа на каждый
 * статус, поэтому новое состояние в контракте сломает сборку, а не тихо останется без кнопки.
 */
const NEXT: Record<DeliveryStatus, DeliveryTransition | undefined> = {
  AWAITING_PVZ: 'AT_PVZ',
  AT_PVZ: 'IN_DELIVERY',
  IN_DELIVERY: 'RECEIVED',
  // Вещь у получателя — дальше двигать нечего.
  RECEIVED: undefined,
}

export const nextStatus = (status: DeliveryStatus): DeliveryTransition | undefined => NEXT[status]

/**
 * Подпись кнопки — то, что сотрудник сейчас делает руками, а не название статуса, в который
 * доставка переедет: за стойкой читают действие.
 */
export const ACTION_LABEL: Record<DeliveryTransition, string> = {
  AT_PVZ: 'Принять в ПВЗ',
  IN_DELIVERY: 'Отправить получателю',
  RECEIVED: 'Выдать получателю',
}
