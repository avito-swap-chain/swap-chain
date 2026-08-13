import type { RecognizedItem } from '@/entities/item'

/**
 * Поля, которые заполняет распознавание. Категории здесь нет: модель отвечает названием
 * из своего промпта, а форме нужен идентификатор справочника — пару им подбирает
 * `matchCategory`, и подставляются они только вместе.
 */
export type RecognizedField = 'title' | 'condition' | 'description'

type Recognized = Partial<Pick<RecognizedItem, RecognizedField>>

/**
 * Что из распознанного встанет в сводку. Правило одно: заполняем только пустые поля.
 * «Заполнить этим» человек нажимает, чтобы закрыть пробелы, а не чтобы отдать модели
 * уже написанное — набранный текст она не трогает.
 *
 * Пустое значение не подставляем вовсе: названия у модели обычно нет, и `undefined`
 * в этом месте стёр бы то, что человек успел написать.
 */
export function recognitionPatch(recognized: Recognized, current: Recognized): Recognized {
  const patch: Recognized = {}

  if (recognized.title && !current.title?.trim()) patch.title = recognized.title
  if (recognized.condition && !current.condition) patch.condition = recognized.condition

  if (recognized.description && !current.description?.trim()) {
    patch.description = recognized.description
  }

  return patch
}
