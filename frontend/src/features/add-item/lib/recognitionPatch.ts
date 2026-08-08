import type { RecognizedItem } from '@/entities/item'

/** Поля формы, которые заполняет распознавание. */
export type RecognizedField = keyof RecognizedItem

/**
 * Что из распознанного можно подставить в форму. Поля, которые человек правил сам, не трогаем:
 * ответ модели приходит с задержкой и не должен затирать уже сделанный выбор.
 */
export function recognitionPatch(
  recognized: RecognizedItem,
  edited: ReadonlySet<RecognizedField>,
): Partial<RecognizedItem> {
  const patch: Partial<RecognizedItem> = {}

  if (!edited.has('title')) patch.title = recognized.title
  if (!edited.has('category')) patch.category = recognized.category
  if (!edited.has('condition')) patch.condition = recognized.condition

  return patch
}
