import { currentPersonaId, PERSONAS } from '@/shared/model/persona'
import type { Item } from '../model/types'
import { itemsOfOthers } from './itemMocks'

/** Больше пяти подсказок не показываем: столько же вариантов принимает форма желания. */
const MAX_SUGGESTIONS = 5

/**
 * Вещь, которую отдают. Не `Item`: подсказки нужны и при публикации новой вещи, когда
 * объявления ещё нет, — общее у обоих входов только название и категория.
 */
export type GivenItem = Pick<Item, 'title' | 'category'>

const norm = (text: string) => text.trim().toLowerCase()

/** Сигнал сработал, если искомое или сохранённое встречается в названии объявления. */
const hitBy = (signal: string) => (item: Item) => norm(item.title).includes(norm(signal))

/**
 * Подсказки «что хочу взамен». Никакой модели за этим не стоит — это эвристика на данных
 * самого пользователя: что он искал и сохранял в избранное, то и предлагаем. Из сигналов
 * оставляем только те, что попадают в реально существующие объявления других пользователей:
 * вариант, которого ни у кого нет, — ребро в никуда, цикл по нему не замкнётся. Если сигналов
 * не хватило, добираем объявлениями из категории отдаваемой вещи.
 *
 * Порядок: избранное → история поиска → добор по категории. Уже выбранные варианты повторно
 * не предлагаем: то же описание — то же ребро графа.
 *
 * Сюда встанет реальный запрос к бэку — снаружи видна только эта функция.
 */
export async function suggestWish(give: GivenItem, chosen: string[] = []): Promise<string[]> {
  const persona = PERSONAS.find((p) => p.id === currentPersonaId())
  if (!persona) return []

  // Саму отдаваемую вещь не предлагаем: менять вещь на такую же незачем.
  const catalog = itemsOfOthers().filter((item) => norm(item.title) !== norm(give.title))
  const signals = [...persona.favorites, ...persona.searches]

  const candidates = [
    ...signals.flatMap((signal) => catalog.filter(hitBy(signal))),
    ...catalog.filter((item) => item.category === give.category),
  ]

  const seen = new Set(chosen.map(norm))
  const suggestions: string[] = []

  for (const item of candidates) {
    if (suggestions.length === MAX_SUGGESTIONS) break
    if (seen.has(norm(item.title))) continue

    seen.add(norm(item.title))
    suggestions.push(item.title)
  }

  return suggestions
}
