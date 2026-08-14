import type { Chain } from '../model/types'

/**
 * Сколько осталось на ответ. У предложения есть срок — сутки с момента, как оно собралось:
 * не ответил никто из участников, и вариант отменяется сам. Без видимого остатка сделка
 * разваливается по молчанию, хотя все были не против, — поэтому срок показывается человеку,
 * а не остаётся полем в контракте.
 *
 * Округляем вниз и в бо́льших единицах: точная минута здесь не нужна, а «осталось 6 ч»
 * читается быстрее, чем «5 ч 47 мин».
 */
export function timeLeft(chain: Chain, now = Date.now()): string | undefined {
  if (chain.status !== 'formed' || !chain.expiresAt) return undefined

  const left = new Date(chain.expiresAt).getTime() - now
  if (Number.isNaN(left)) return undefined
  if (left <= 0) return 'время на ответ вышло'

  const hours = Math.floor(left / 3_600_000)
  if (hours >= 1) return `ответить осталось ${hours} ч`

  // Меньше минуты — всё равно «1 мин»: ноль читается как «уже поздно», а ответить ещё можно.
  const minutes = Math.max(1, Math.floor(left / 60_000))
  return `ответить осталось ${minutes} мин`
}
