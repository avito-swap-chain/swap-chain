import { unwrap } from '@/shared/api/fetcher'
import {
  confirmAdminParticipantReceipt,
  getAdminChain,
  listAdminChains,
  listAdminDeliveries,
  transitionAdminDelivery,
} from '@/shared/api/generated/endpoints'
import type {
  AdminChain,
  AdminChainList,
  AdminChainSummary,
  AdminDeliveryList,
} from '@/shared/api/generated/model'
import type { DeliveryTransition } from '../lib/transitions'
import type { Delivery } from '../model/types'

/** Ключи кэша. Список один: фильтра по статусу и страниц у экрана нет. */
export const deliveryKeys = {
  list: () => ['admin-deliveries'] as const,
  chains: () => ['admin-chains'] as const,
  chain: (id: number) => ['admin-chains', id] as const,
}

/**
 * Очередь ПВЗ по цепочкам, а не по отдельным доставкам. Так её и видит сотрудник за стойкой:
 * вещи ездят не сами по себе, а кругом на три человека, и «принять» одну из них имеет смысл
 * только вместе с остальными — цепочка закрывается, когда все получили своё.
 *
 * Плоский список доставок оказался нечитаемым ровно поэтому: в нём три строки одной сделки
 * стояли вперемешку с чужими, и понять, чего ждёт конкретный круг, было нельзя.
 */
export async function getChains(): Promise<AdminChainSummary[]> {
  const { chains } = unwrap<AdminChainList>(await listAdminChains({ status: 'ACCEPTED' }))
  return chains
}

/** Цепочка целиком: каждая передача внутри круга со своим статусом. */
export async function getChain(chainId: number): Promise<AdminChain> {
  return unwrap<AdminChain>(await getAdminChain(chainId))
}

/**
 * Отметить, что участник получил свою вещь. Раньше это мог сделать только он сам из кабинета,
 * и на стойке круг застревал: человек ушёл с вещью, а цепочка ждала его подтверждения.
 * Теперь отметку ставит сотрудник — он и выдаёт вещь.
 */
export async function confirmParticipantReceipt(
  chainId: number,
  participantId: number,
): Promise<void> {
  unwrap(await confirmAdminParticipantReceipt(chainId, participantId))
}

/**
 * Мок-режима у админки нет, в отличие от остальных разделов. Демо на моках открывается
 * без бэкенда, но роль там взять неоткуда — «я» на моках это выбранная персона, а не сессия
 * с `ADMIN`. Значит и ссылки в меню не будет, и вторая машина состояний доставки в моках
 * осталась бы кодом, который в демо никто не откроет. Админка — инструмент сотрудника ПВЗ,
 * а не часть продукта, который смотрит жюри.
 */
export async function getDeliveries(): Promise<Delivery[]> {
  // Курсор ответа не используем: за стойкой разбирают текущую очередь, а не листают историю.
  const { deliveries } = unwrap<AdminDeliveryList>(await listAdminDeliveries())
  return deliveries
}

/** Перевести доставку на следующий шаг пути. Повтор текущего статуса бэкенд принимает молча. */
export async function advanceDelivery(id: number, status: DeliveryTransition): Promise<Delivery> {
  return unwrap<Delivery>(await transitionAdminDelivery(id, { status }))
}
