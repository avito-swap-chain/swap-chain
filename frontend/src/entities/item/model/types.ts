export type ItemCondition = 'new' | 'good' | 'used'

/** Статус вещи в обороте обмена. */
export type ItemStatus = 'idle' | 'searching' | 'reserved'

/** Один вариант желания — одно ребро графа, по которому ищется цепочка. */
export interface Wish {
  category: string
  description: string
}

export interface Item {
  id: string
  title: string
  /**
   * Категория и состояние приходят не всегда: в контракте бэкенда (0.4.0) таких полей нет,
   * и по данным из API они пустые. Интерфейс это учитывает и не рисует пустую подпись —
   * додумывать состояние за пользователя нельзя, в обмене оно и есть предмет разговора.
   */
  category: string
  condition?: ItemCondition
  photoUrl?: string
  /**
   * Варианты желания: «PS5 или Xbox или беговая дорожка» — три ребра и три шанса
   * замкнуть цикл. Пустой массив = обмен на вещь не включён.
   */
  wish: Wish[]
  status: ItemStatus
}
