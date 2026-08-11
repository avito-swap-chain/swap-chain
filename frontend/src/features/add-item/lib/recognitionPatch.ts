import type { RecognizedItem } from '@/entities/item'

/** Поля формы, которые заполняет распознавание. */
export type RecognizedField = 'title' | 'category' | 'condition' | 'description'

type Recognized = Partial<Pick<RecognizedItem, RecognizedField>>

/**
 * Что из распознанного можно подставить в форму. Поля, которые человек правил сам, не трогаем:
 * ответ модели приходит с задержкой и не должен затирать уже сделанный выбор.
 *
 * Пустое значение не подставляем вовсе: модель не даёт названия, и `undefined` в этом месте
 * стёр бы то, что человек уже написал.
 */
export function recognitionPatch(
  recognized: Recognized,
  edited: ReadonlySet<RecognizedField>,
): Recognized {
  const patch: Recognized = {}

  if (recognized.title && !edited.has('title')) patch.title = recognized.title
  if (recognized.category && !edited.has('category')) patch.category = recognized.category
  if (recognized.condition && !edited.has('condition')) patch.condition = recognized.condition
  if (recognized.description && !edited.has('description')) {
    patch.description = recognized.description
  }

  return patch
}
