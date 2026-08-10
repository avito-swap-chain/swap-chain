/** Насколько описание годится для подбора: от «не за что зацепиться» до «достаточно». */
export type DescriptionQuality = 'short' | 'fair' | 'good'

/**
 * Пороги длины — от бэка (10 авг). Считает их фронт: ручки под это нет и не нужно, правило
 * целиком в длине текста. `quality_score` из фото-анализа сюда не относится — это оценка
 * состояния вещи по картинке, бэковая метрика, на форме её не показываем.
 *
 * Границы взяты включительно снизу: 20 символов — уже не «слишком коротко», 45 — уже «хорошо».
 */
const SHORT_UP_TO = 20
const FAIR_UP_TO = 45

export function descriptionQuality(description: string): DescriptionQuality {
  const length = description.trim().length

  if (length < SHORT_UP_TO) return 'short'
  if (length < FAIR_UP_TO) return 'fair'
  return 'good'
}
