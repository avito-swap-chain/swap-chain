import type { AdminDeliveryStatus } from '@/shared/api/generated/model'

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
  /**
   * Где физически вещь, которую участник получает. Приехало в контракте 0.8.0 и закрывает
   * то, на что указал ментор: до этого стадия передачи молчала, пока кто-нибудь не отметит
   * получение, хотя вещь всё это время двигалась по пунктам выдачи.
   *
   * Тип берём из контракта, а не из `entities/delivery`: сущности одного слоя друг друга
   * не импортируют, а переводить статус доставки во второй раз незачем — он и там оставлен
   * как есть.
   */
  incomingDelivery?: AdminDeliveryStatus
  isMe?: boolean
}

export interface Chain {
  id: string
  status: ChainStatus
  /** Участники в порядке обхода цикла: participants[i] отдаёт вещь participants[i+1]. */
  participants: ChainParticipant[]
  /**
   * Когда предложение протухнет: сутки с момента, как оно собралось. Дальше бэкенд
   * отменяет его сам. Показываем остаток человеку — иначе вариант, который всех устраивал,
   * разваливается по молчанию.
   */
  expiresAt?: string
  /**
   * Только у `cancelled`: вещь, из-за которой предложение отменилось — она ушла
   * в собравшуюся цепочку. Без неё отмену не объяснить, а необъяснённая выглядит поломкой.
   */
  cancelledItemId?: string
}
