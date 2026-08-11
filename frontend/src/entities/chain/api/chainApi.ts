import { unwrap } from '@/shared/api/fetcher'
import {
  confirmChainReceipt,
  getChain as getChainRequest,
  listChains,
  submitChainDecision,
} from '@/shared/api/generated/endpoints'
import type { Chain as ApiChain, ChainList } from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import { currentUserId } from '@/shared/model/session'
import type { Chain, ChainDecision } from '../model/types'
import * as mock from './chainMocks'
import { mapChain } from './mapChain'

/**
 * Ключи кэша TanStack Query. Иерархия не случайна: `my` — префикс `detail`,
 * поэтому инвалидация `all` после действия обновляет и цепочку, и список обменов.
 */
export const chainKeys = {
  all: ['chains'] as const,
  my: () => chainKeys.all,
  detail: (id: string) => [...chainKeys.all, id] as const,
}

/** Обмены, в которых участвует текущий пользователь, — чужие в кабинет не попадают. */
export async function getMyChains(): Promise<Chain[]> {
  if (!isBackendConnected) return mock.myChains()

  const meId = await currentUserId()
  const { chains } = unwrap<ChainList>(await listChains())
  return chains.map((chain) => mapChain(chain, meId))
}

export async function getChain(id: string): Promise<Chain> {
  if (!isBackendConnected) return mock.chainById(id)

  const meId = await currentUserId()
  return mapChain(unwrap<ApiChain>(await getChainRequest(Number(id))), meId)
}

/**
 * Ответ на предложение. Лайк — «вариант подходит»: обмен стартует, только когда лайкнули все,
 * до этого вещь остаётся у владельца и участвует в других вариантах.
 *
 * Дизлайк распускает цепочку целиком. Замену вышедшему сервис не ищет: в цепочке максимум
 * три участника, и собрать новый вариант с нуля дешевле, чем латать старый — часть тех же
 * людей в него обычно и попадает. Отказ от варианта при этом не равен отказу от обмена:
 * вещь остаётся свободной и участвует в других вариантах.
 */
export async function respondToChain(id: string, decision: ChainDecision): Promise<void> {
  if (!isBackendConnected) return mock.respond(id, decision)

  unwrap(
    await submitChainDecision(Number(id), {
      decision: decision === 'like' ? 'APPROVED' : 'DECLINED',
    }),
  )
}

/** Выйти из цепочки до общего подтверждения — вещь снова свободна. */
export async function leaveChain(id: string): Promise<void> {
  if (!isBackendConnected) return mock.leave(id)

  // Отдельной ручки выхода в контракте нет: выход — это то же решение `DECLINED`,
  // что и дизлайк, и цепочка распускается целиком.
  unwrap(await submitChainDecision(Number(id), { decision: 'DECLINED' }))
}

/**
 * Подтвердить получение вещи. Отмечается только текущий пользователь: цепочка закроется,
 * когда получение подтвердят все — обмен состоялся только тогда, когда его закрыли с обеих сторон.
 */
export async function confirmReceipt(id: string): Promise<void> {
  if (!isBackendConnected) return mock.confirmReceipt(id)

  // Какую именно вещь получает пользователь, бэкенд выбирает сам по сессии — подтвердить
  // чужую передачу нельзя. Отметка принимается только после того, как сотрудник ПВЗ отправил
  // вещь получателю (`IN_DELIVERY`), иначе 409: подтверждать нечего, пока она не выехала.
  unwrap(await confirmChainReceipt(Number(id)))
}

/**
 * Вещь сняли с обмена — предложения с ней больше не соберутся. Только для моков: на бэкенде
 * это делает он сам и присылает `chain.rejected` с причиной `item_withdrawn`.
 *
 * Живёт в `entities/chain`, потому что трогает цепочки; вызывает её фича снятия — ей можно
 * знать про обе сущности сразу, а сущностям друг про друга нет.
 */
export const dissolveChainsWithItem = mock.dissolveWithItem
