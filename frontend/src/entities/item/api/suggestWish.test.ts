import { beforeEach, describe, expect, it } from 'vitest'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import { getMyItems } from './itemsApi'
import { suggestWish } from './suggestWish'

const [DASHA, MARK, LENA] = PERSONAS

const MONITOR = { title: 'Монитор LG 27" IPS', category: 'Электроника' }

const descriptions = async (...args: Parameters<typeof suggestWish>) =>
  (await suggestWish(...args)).map((wish) => wish.description)

/** Названия объявлений персоны — чтобы проверять подсказки против реального каталога. */
const titlesOf = async (personaId: string) => {
  usePersonaStore.setState({ personaId })
  return (await getMyItems()).map((item) => item.title)
}

describe('suggestWish — подсказки желания из поисков и избранного', () => {
  beforeEach(() => {
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('предлагает только вещи, которые действительно есть у других пользователей', async () => {
    const others = [...(await titlesOf(MARK.id)), ...(await titlesOf(LENA.id))]
    usePersonaStore.setState({ personaId: DASHA.id })

    const suggested = await descriptions(MONITOR)

    expect(suggested.length).toBeGreaterThanOrEqual(3)
    suggested.forEach((title) => expect(others).toContain(title))
  })

  it('сигнал, которому не соответствует ни одно объявление, подсказкой не становится', async () => {
    // Даша искала фитнес-браслет, но такого объявления нет ни у кого — ребро в никуда.
    const suggested = await descriptions(MONITOR)

    expect(suggested.some((title) => title.toLowerCase().includes('браслет'))).toBe(false)
  })

  it('избранное идёт первым, за ним история поиска', async () => {
    const suggested = await descriptions(MONITOR)

    expect(suggested[0]).toBe('PlayStation 4 Slim, 1 ТБ')
    expect(suggested).toContain('Умные часы Amazfit GTR')
  })

  it('свои объявления в подсказки не попадают', async () => {
    const mine = await titlesOf(DASHA.id)

    const suggested = await descriptions(MONITOR)

    mine.forEach((title) => expect(suggested).not.toContain(title))
  })

  it('саму отдаваемую вещь не предлагает', async () => {
    // Наушники есть у Марка и лежат у Даши в категории подсказок — но менять их на них же незачем.
    const suggested = await descriptions({ title: 'Наушники', category: 'Аудио' })

    expect(suggested).not.toContain('Наушники')
  })

  it('добирает объявлениями из категории отдаваемой вещи', async () => {
    // Плёночный фотоаппарат Лены не попадает ни в один поиск Даши — только по категории.
    const electronics = await descriptions(MONITOR)
    const sport = await descriptions({ title: 'Гантели 20 кг', category: 'Спорт и отдых' })

    expect(electronics).toContain('Плёночный фотоаппарат')
    expect(sport).not.toContain('Плёночный фотоаппарат')
  })

  it('уже выбранные варианты повторно не предлагает', async () => {
    const suggested = await descriptions(MONITOR)

    const rest = await descriptions(MONITOR, [
      { category: 'Электроника', description: suggested[0] },
      { category: 'Электроника', description: suggested[1].toUpperCase() },
    ])

    expect(rest).not.toContain(suggested[0])
    expect(rest).not.toContain(suggested[1])
  })

  it('вариантов не больше пяти', async () => {
    usePersonaStore.setState({ personaId: LENA.id })

    // У Лены кандидатов шесть: четыре по поискам и избранному плюс добор по категории.
    const suggested = await descriptions({
      title: 'PlayStation 4 Slim, 1 ТБ',
      category: 'Электроника',
    })

    expect(suggested).toHaveLength(5)
    expect(suggested).not.toContain('Механическая клавиатура')
  })

  it('один и тот же вход даёт один и тот же ответ', async () => {
    expect(await suggestWish(MONITOR)).toEqual(await suggestWish(MONITOR))
  })
})
