import { beforeEach, describe, expect, it } from 'vitest'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import { createItem, getMyItems, setItemWish } from './itemsApi'

const [DASHA, MARK] = PERSONAS

const wish = [{ category: 'Электроника', description: 'Монитор 27"' }]

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
    const updated = await setItemWish('1', [{ category: 'Аудио', description: 'Колонка' }])

    expect(updated.status).toBe('reserved')
    expect(updated.wish[0].description).toBe('Колонка')
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
    const variants = [
      { category: 'Электроника', description: 'Игровая приставка' },
      { category: 'Электроника', description: 'Смартфон' },
      { category: 'Спорт и отдых', description: 'Беговая дорожка' },
    ]

    await setItemWish('3', variants)

    expect((await itemOf('3'))?.wish).toEqual(variants)
  })

  it('пустые варианты отсеиваются, описания обрезаются по краям', async () => {
    const updated = await setItemWish('3', [
      { category: 'Электроника', description: '  Монитор 27"  ' },
      { category: 'Аудио', description: '   ' },
      { category: 'Транспорт', description: '' },
    ])

    expect(updated.wish).toEqual([{ category: 'Электроника', description: 'Монитор 27"' }])
  })

  it('когда все варианты пустые, включать обмен нечего — ребро графа не построить', async () => {
    await expect(
      setItemWish('3', [
        { category: 'Электроника', description: '   ' },
        { category: 'Аудио', description: '' },
      ]),
    ).rejects.toThrow()
  })

  it('желание без единого варианта отклоняется', async () => {
    await expect(setItemWish('3', [])).rejects.toThrow()
  })

  it('повтор того же описания — то же ребро графа, второй раз не сохраняется', async () => {
    const updated = await setItemWish('3', [
      { category: 'Электроника', description: 'Монитор 27"' },
      { category: 'Аудио', description: 'монитор 27"' },
    ])

    expect(updated.wish).toEqual([{ category: 'Электроника', description: 'Монитор 27"' }])
  })
})

describe('createItem — публикация вещи сразу с желанием', () => {
  const draft = {
    title: 'Ролики',
    category: 'Спорт и отдых',
    condition: 'good' as const,
    wish: [
      { category: 'Аудио', description: 'Колонка' },
      { category: 'Электроника', description: '  Электронная книга ' },
      { category: 'Дом и дача', description: '  ' },
    ],
  }

  beforeEach(() => {
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('новая вещь уходит в подбор со всеми непустыми вариантами', async () => {
    const created = await createItem(draft)

    expect(created.status).toBe('searching')
    expect(created.wish).toEqual([
      { category: 'Аудио', description: 'Колонка' },
      { category: 'Электроника', description: 'Электронная книга' },
    ])
    expect((await itemOf(created.id))?.wish).toHaveLength(2)
  })

  it('файл фотографии в вещь не попадает: он нужен только загрузке на бэкенд', async () => {
    const file = new File(['photo'], 'bike.jpg', { type: 'image/jpeg' })
    const created = await createItem({ ...draft, photoUrl: 'blob:preview', photoFile: file })

    expect(created).not.toHaveProperty('photoFile')
    expect(created.photoUrl).toBe('blob:preview')
  })

  it('вещь без единого непустого варианта не публикуется', async () => {
    await expect(
      createItem({ ...draft, wish: [{ category: 'Аудио', description: ' ' }] }),
    ).rejects.toThrow()
  })
})
