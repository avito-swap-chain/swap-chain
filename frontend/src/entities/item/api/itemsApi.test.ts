import { beforeEach, describe, expect, it } from 'vitest'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import { createItem, editItem, getMyItems, setItemWish, withdrawItem } from './itemsApi'

const [DASHA, MARK] = PERSONAS

const wish = ['Монитор 27"']

const itemOf = async (id: string) => (await getMyItems()).find((item) => item.id === id)

describe('setItemWish — включение обмена у размещённого объявления', () => {
  beforeEach(() => {
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('объявление без обмена уходит в подбор цепочки', async () => {
    const updated = await setItemWish('3', wish)

    expect(updated.status).toBe('searching')
    expect(updated.wish).toEqual(wish)
  })

  it('изменение сохраняется в кабинете, а не только в ответе', async () => {
    await setItemWish('3', wish)

    expect((await itemOf('3'))?.status).toBe('searching')
  })

  it('вещь, уже попавшую в цепочку, обмен не расколдовывает', async () => {
    // '1' — велосипед Даши, он зарезервирован в цепочке c1.
    const updated = await setItemWish('1', ['Колонка'])

    expect(updated.status).toBe('reserved')
    expect(updated.wish[0]).toBe('Колонка')
  })

  it('чужое объявление не тронуть: id ищется в кабинете текущей персоны', async () => {
    // '23' — часы Марка; Даша про них знать не должна.
    await expect(setItemWish('23', wish)).rejects.toThrow()

    usePersonaStore.setState({ personaId: MARK.id })
    await expect(setItemWish('23', wish)).resolves.toMatchObject({ status: 'searching' })
  })
})

describe('варианты желания — каждый отдельное ребро графа', () => {
  beforeEach(() => {
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('сохраняются все варианты и их порядок', async () => {
    const variants = ['Игровая приставка', 'Смартфон', 'Беговая дорожка']

    await setItemWish('3', variants)

    expect((await itemOf('3'))?.wish).toEqual(variants)
  })

  it('пустые варианты отсеиваются, описания обрезаются по краям', async () => {
    const updated = await setItemWish('3', ['  Монитор 27"  ', '   ', ''])

    expect(updated.wish).toEqual(['Монитор 27"'])
  })

  it('когда все варианты пустые, включать обмен нечего — ребро графа не построить', async () => {
    await expect(setItemWish('3', ['   ', ''])).rejects.toThrow()
  })

  it('желание без единого варианта отклоняется', async () => {
    await expect(setItemWish('3', [])).rejects.toThrow()
  })

  it('повтор того же описания — то же ребро графа, второй раз не сохраняется', async () => {
    const updated = await setItemWish('3', ['Монитор 27"', 'монитор 27"'])

    expect(updated.wish).toEqual(['Монитор 27"'])
  })
})

describe('editItem — правка размещённого объявления', () => {
  beforeEach(() => {
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('описание и желание меняются вместе, снимать вещь с обмена для этого не нужно', async () => {
    await setItemWish('3', wish)
    const edited = await editItem('3', {
      wish: ['Колонка'],
      description: 'Монитор LG, 27 дюймов, IPS-матрица',
    })

    expect(edited.status).toBe('searching')
    expect(edited.description).toBe('Монитор LG, 27 дюймов, IPS-матрица')
    expect(edited.wish[0]).toBe('Колонка')
  })

  it('пустое описание ничего не стирает: у бэкенда это поле нельзя очистить', async () => {
    await editItem('3', { wish, description: 'Было описание' })
    const edited = await editItem('3', { wish, description: '   ' })

    expect(edited.description).toBe('Было описание')
  })
})

describe('withdrawItem — снятие с обмена', () => {
  beforeEach(() => {
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('вещь остаётся в кабинете, но без желания и вне подбора', async () => {
    await setItemWish('3', wish)
    const withdrawn = await withdrawItem('3')

    expect(withdrawn.status).toBe('withdrawn')
    expect(withdrawn.wish).toEqual([])
    // Не удаление: объявление никуда не делось, его снова можно отдать в обмен.
    expect(await itemOf('3')).toMatchObject({ status: 'withdrawn', title: withdrawn.title })
  })

  it('снятую вещь можно вернуть в обмен', async () => {
    await setItemWish('3', wish)
    await withdrawItem('3')

    expect((await setItemWish('3', wish)).status).toBe('searching')
  })

  it('чужое объявление снять нельзя', async () => {
    await expect(withdrawItem('23')).rejects.toThrow()
  })
})

describe('createItem — публикация вещи сразу с желанием', () => {
  const draft = {
    title: 'Ролики',
    category: 'Спорт и отдых',
    condition: 'good' as const,
    wish: ['Колонка', '  Электронная книга ', '  '],
  }

  beforeEach(() => {
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('новая вещь уходит в подбор со всеми непустыми вариантами', async () => {
    const created = await createItem(draft)

    expect(created.status).toBe('searching')
    expect(created.wish).toEqual(['Колонка', 'Электронная книга'])
    expect((await itemOf(created.id))?.wish).toHaveLength(2)
  })

  it('файл фотографии в вещь не попадает: он нужен только загрузке на бэкенд', async () => {
    const file = new File(['photo'], 'bike.jpg', { type: 'image/jpeg' })
    const created = await createItem({ ...draft, photoUrl: 'blob:preview', photoFile: file })

    expect(created).not.toHaveProperty('photoFile')
    expect(created.photoUrl).toBe('blob:preview')
  })

  it('вещь без единого непустого варианта не публикуется', async () => {
    await expect(createItem({ ...draft, wish: [' '] })).rejects.toThrow()
  })
})
