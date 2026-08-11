/**
 * Состояние цепочки обмена (машина состояний сделки).
 * `cancelled` — предложение больше не собрать: одна из его вещей ушла в другую цепочку.
 * Это не распад (`dissolved`): там от обмена отказался человек, здесь его просто опередили.
 */
export type ChainStatus = 'formed' | 'active' | 'completed' | 'dissolved' | 'cancelled'

/** Решение конкретного участника по цепочке: лайк даёт `confirmed`, дизлайк — `declined`. */
export type ParticipantStatus = 'pending' | 'confirmed' | 'declined'

/**
 * Ответ участника на предложение: лайк — «этот вариант мне подходит», дизлайк — отказ
 * от этого варианта, а не от обмена вообще: вариант распадётся, но вещь останется
 * в подборе. Игнор — просто отсутствие ответа.
 */
export type ChainDecision = 'like' | 'dislike'

export interface ChainParticipant {
  userId: string
  name: string
  avatarUrl?: string
  rating?: number
  /** Вещь, которую участник отдаёт следующему по кругу. */
  givesItem: { id: string; title: string; photoUrl?: string }
  status: ParticipantStatus
  /**
   * Отметил ли участник, что получил свою вещь. Флаг у участника, а не у цепочки:
   * иначе на стадии передачи не отличить «я подтвердил, жду остальных» от «мне надо нажать кнопку».
   */
  receiptConfirmed: boolean
  isMe?: boolean
}

export interface Chain {
  id: string
  status: ChainStatus
  /** Участники в порядке обхода цикла: participants[i] отдаёт вещь participants[i+1]. */
  participants: ChainParticipant[]
  /**
   * Только у `cancelled`: вещь, из-за которой предложение отменилось — она ушла
   * в собравшуюся цепочку. Без неё отмену не объяснить, а необъяснённая выглядит поломкой.
   */
  cancelledItemId?: string
}
