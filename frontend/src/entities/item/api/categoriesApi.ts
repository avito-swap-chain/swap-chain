import { unwrap } from '@/shared/api/fetcher'
import { listCategories } from '@/shared/api/generated/endpoints'
import type { CategoryList } from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import { CATEGORIES } from '../model/dictionaries'
import type { Category } from '../model/types'

export const categoryKeys = { list: () => ['categories'] as const }

/**
 * Имя категории у бэкенда — это ещё и словарь для матчинга: «Электроника: смартфон, телефон,
 * планшет, ноутбук…». Человеку в выпадающем списке нужна только первая часть, всё после
 * двоеточия — подсказки модели, а не подпись.
 */
const shortName = (name: string) => name.split(':')[0].trim()

/**
 * Категории вещей. С бэкендом их девять и они же участвуют в подборе — свой список держать
 * нельзя, иначе человек выбирает из одного, а обмен ищется по другому.
 *
 * На моках остаётся собственный словарь: демо открывается без сервера, и категории там
 * нужны для тех же карточек.
 */
export async function getCategories(): Promise<Category[]> {
  if (!isBackendConnected) {
    return CATEGORIES.map((name, index) => ({ id: index + 1, name }))
  }

  const { categories } = unwrap<CategoryList>(await listCategories())

  return categories.map((category) => ({ id: category.id, name: shortName(category.name) }))
}

/**
 * Категория, предложенная моделью по фотографии, в списке бэкенда. Модель отвечает названием
 * из своего промпта, а не идентификатором, и совпасть оно может не всегда: подсказка
 * без пары просто не подставляется — описание и состояние от этого не страдают.
 */
export const matchCategory = (name: string | undefined, categories: Category[]) =>
  categories.find((category) => category.name.toLowerCase() === name?.trim().toLowerCase())
