import { unwrap } from '@/shared/api/fetcher'
import { listAdminDeliveries, transitionAdminDelivery } from '@/shared/api/generated/endpoints'
import type { AdminDeliveryList } from '@/shared/api/generated/model'
import type { DeliveryTransition } from '../lib/transitions'
import type { Delivery } from '../model/types'

/** Ключи кэша. Список один: фильтра по статусу и страниц у экрана нет. */
export const deliveryKeys = { list: () => ['admin-deliveries'] as const }

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
