import type { Chain, ChainParticipant } from '../model/types'

/** Участник, за которого играет текущий пользователь. */
export const findMe = (chain: Chain): ChainParticipant | undefined =>
  chain.participants.find((p) => p.isMe)

/** Подпись участника: себя человек читает как «Вы» — это следует из `isMe`, а не из имени. */
export const displayName = (participant: ChainParticipant): string =>
  participant.isMe ? 'Вы' : participant.name

/**
 * Ход за пользователем: цепочка не поедет, пока он не ответит на предложение,
 * а после старта — пока не отметит получение вещи. Передача — такой же его ход,
 * как и согласование, поэтому обмен зовёт к себе на обеих стадиях.
 */
export const needsMyAction = (chain: Chain): boolean => {
  const me = findMe(chain)
  if (!me) return false

  if (chain.status === 'formed') return me.status === 'pending'
  if (chain.status === 'active') return !me.receiptConfirmed
  return false
}

/** Сколько участников уже подтвердили участие — числитель прогресса «N из M». */
export const countConfirmed = (chain: Chain): number =>
  chain.participants.filter((p) => p.status === 'confirmed').length

/** Сколько участников уже отметили получение — числитель прогресса на стадии передачи. */
export const countReceipts = (chain: Chain): number =>
  chain.participants.filter((p) => p.receiptConfirmed).length

/**
 * Участник отметил получение вещи. Цепочка закрывается только тогда, когда получение
 * подтвердили все: односторонняя отметка — это ещё не состоявшийся обмен.
 */
export const confirmReceiptFor = (chain: Chain, userId: string): Chain => {
  const participants = chain.participants.map((p) =>
    p.userId === userId ? { ...p, receiptConfirmed: true } : p,
  )

  return {
    ...chain,
    status: participants.every((p) => p.receiptConfirmed) ? 'completed' : chain.status,
    participants,
  }
}

/** Участник, отказавшийся от обмена, — из-за него цепочка распалась. */
export const findDecliner = (chain: Chain): ChainParticipant | undefined =>
  chain.participants.find((p) => p.status === 'declined')

export interface Neighbours {
  /** Кому я отдаю свою вещь (следующий по кругу). */
  receiver: ChainParticipant
  /** От кого получаю вещь (предыдущий по кругу). */
  giver: ChainParticipant
}

/**
 * Сосед по кругу — реальный контрагент: ему пользователь отдаёт вещь или от него получает,
 * через пункт выдачи. Остальные объясняют механику («круг замкнут»), но
 * подробности о них пользователю не нужны — отсюда разная громкость в интерфейсе.
 */
export const isNeighbour = (
  participant: ChainParticipant,
  { receiver, giver }: Neighbours,
): boolean => participant.userId === receiver.userId || participant.userId === giver.userId

/** Соседи по кругу обмена. `undefined`, если текущего пользователя нет в цепочке. */
export function findNeighbours(chain: Chain): Neighbours | undefined {
  const index = chain.participants.findIndex((p) => p.isMe)
  if (index === -1) return undefined

  const total = chain.participants.length
  return {
    receiver: chain.participants[(index + 1) % total],
    giver: chain.participants[(index + total - 1) % total],
  }
}
