import { describe, expect, it } from 'vitest'
import { recognize as recognizeMock } from './itemMocks'

/** Задержку в тестах убираем: состояние «распознаём» нужно интерфейсу, а не проверке. */
const recognize = (name: string) => recognizeMock(new File([], name), 0)

describe('распознавание вещи по фото на моках', () => {
  it('узнаёт демо-фотографию и заполняет все три поля', async () => {
    expect(await recognize('bike.jpg')).toEqual({
      title: 'Горный велосипед',
      category: 'Спорт и отдых',
      condition: 'good',
    })
  })

  it('не путает более точное совпадение с более коротким', async () => {
    const wide = await recognize('monitor24.jpg')
    const narrow = await recognize('monitor.jpg')

    expect(wide.title).toBe('Монитор 24"')
    expect(narrow.title).toBe('Монитор LG 27" IPS')
  })

  it('узнаёт файл независимо от регистра и пути', async () => {
    const recognized = await recognize('/mock/items/HeadPhones.JPG')

    expect(recognized.title).toBe('Наушники')
  })

  it('на незнакомом фото отказывается, а не выдумывает название', async () => {
    await expect(recognize('IMG_2245.jpg')).rejects.toThrow('Не удалось распознать')
  })
})
