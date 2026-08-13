import { describe, expect, it } from 'vitest'
import { recognitionPatch } from './recognitionPatch'

const RECOGNIZED = {
  title: 'Горный велосипед',
  condition: 'good',
  description: 'Рама 19", катался два сезона',
} as const

describe('recognitionPatch — подстановка распознанного в сводку', () => {
  it('заполняет пустые поля целиком', () => {
    expect(recognitionPatch(RECOGNIZED, {})).toEqual(RECOGNIZED)
  })

  it('не перетирает то, что человек уже написал', () => {
    const patch = recognitionPatch(RECOGNIZED, { title: 'Велосипед Stels' })

    expect(patch.title).toBeUndefined()
    expect(patch.description).toBe(RECOGNIZED.description)
  })

  // Пробелы в поле — это пустое поле: иначе случайный пробел лишал бы человека подсказки.
  it('поле из одних пробелов считается пустым', () => {
    expect(recognitionPatch(RECOGNIZED, { title: '   ' }).title).toBe('Горный велосипед')
  })

  it('если заполнено всё, подставлять нечего', () => {
    expect(recognitionPatch(RECOGNIZED, RECOGNIZED)).toEqual({})
  })

  it('чего модель не дала, того и не подставляем', () => {
    expect(recognitionPatch({ description: 'Тёмный деним' }, {})).toEqual({
      description: 'Тёмный деним',
    })
  })
})
