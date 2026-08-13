import type { Chain } from '../model/types'
import { findMe } from './participants'

/**
 * Предложение, ждущее ответа: цепочка ещё собирается, а я по ней не высказался.
 * Таких у человека сразу несколько — подбор параллельный, и это отдельный блок раздела.
 */
export const isOpenOffer = (chain: Chain): boolean =>
  chain.status === 'formed' && findMe(chain)?.status === 'pending'

/**
 * Что попадает во «Входящие»: варианты, ждущие ответа, и отменившиеся. Вторые остаются,
 * пока их не объяснили, — иначе вариант просто исчезает, и человек решает, что сервис
 * его обманул. Признак нужен и списку, и счётчику на вкладке, поэтому живёт в сущности.
 */
export const isInboxOffer = (chain: Chain): boolean =>
  isOpenOffer(chain) || chain.status === 'cancelled'

/**
 * Живой вариант: цепочка ещё собирается и я из неё не выходил. Лайк вариант не закрывает —
 * вещь блокируется, только когда согласятся все, поэтому лайкнутое тоже конкурирует за вещь.
 */
const isLive = (chain: Chain): boolean =>
  chain.status === 'formed' && findMe(chain)?.status !== 'declined'

/**
 * Сколько живых вариантов держат одну и ту же мою вещь. Больше одного — конкуренция:
 * состоится ровно один, остальные отменятся. Считается на фронте: бэкенд отдаёт цепочки
 * по отдельности и про соперничество между ними в ответе ничего не говорит.
 */
export const countVariantsWithItem = (chains: Chain[], itemId: string): number =>
  chains.filter((chain) => isLive(chain) && findMe(chain)?.givesItem.id === itemId).length

/** Почему предложение отменилось — одной фразой: чья вещь ушла в собравшуюся цепочку. */
export function cancelReason(chain: Chain): string {
  const taken = chain.participants.find((p) => p.givesItem.id === chain.cancelledItemId)
  if (!taken) return 'Одна из вещей этого варианта ушла в другой обмен'

  return taken.isMe
    ? `Ваша вещь «${taken.givesItem.title}» ушла в другой обмен`
    : `Вещь «${taken.givesItem.title}» участника ${taken.name} ушла в другой обмен`
}
