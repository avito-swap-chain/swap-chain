/**
 * Профиль пользователя — то, что видно про человека до обмена. Участники цепочки
 * денормализованы внутрь `Chain` (имя и аватар приходят вместе с ней), а эта сущность
 * про отдельный экран: свой профиль правят, чужой открывают, чтобы понять, с кем меняются.
 */
export interface Profile {
  id: string
  name: string
  avatarUrl?: string
  /** С какого времени человек на сервисе: единственный довод, который сейчас отдаёт бэкенд. */
  registeredAt: string
  /**
   * Рейтинг, число отзывов и завершённых обменов. С контракта 0.8.0 приходят с бэкенда,
   * до этого жили только на моках. Рейтинг остаётся необязательным и после: пока человека
   * никто не оценил, его нет вовсе — и звёзды рисовать нечем, а «5,0» без единого отзыва
   * врал бы ровно там, где на него смотрят.
   */
  rating?: number
  reviews?: number
  completedExchanges?: number
}

/** Отзыв о человеке: их оставляют соседи по кругу после завершённого обмена. */
export interface Review {
  id: string
  author: { id: string; name: string; avatarUrl?: string }
  rating: number
  text?: string
  createdAt: string
}

/** Что человек меняет в своём профиле. Пустая строка у аватара — «убрать фотографию». */
export interface ProfileEdit {
  name?: string
  avatarUrl?: string
}

/** Причина жалобы. Подписи — в `model/dictionaries`. */
export type ReportReason = 'item' | 'noshow' | 'rude' | 'fraud' | 'other'

/**
 * Жалоба на человека. Привязана к обмену, если он был: разбирать «нагрубил» в отрыве
 * от конкретной цепочки нечем — переписка и вещи живут именно там.
 */
export interface Report {
  targetUserId: string
  chainId?: string
  reason: ReportReason
  text?: string
}

/** Строка чёрного списка: кого человек заблокировал. Имя нужно, чтобы список читался. */
export interface BlockedUser {
  id: string
  name: string
}
