import type { ItemCondition } from './types'

/** Категории вещей и желаний — общий словарь форм и карточек. */
export const CATEGORIES = [
  'Электроника',
  'Аудио',
  'Спорт и отдых',
  'Транспорт',
  'Одежда и обувь',
  'Дом и дача',
  'Хобби и творчество',
] as const

export const CONDITION_LABEL: Record<ItemCondition, string> = {
  new: 'новое',
  good: 'хорошее',
  used: 'б/у',
}

export const CONDITIONS = Object.keys(CONDITION_LABEL) as ItemCondition[]
