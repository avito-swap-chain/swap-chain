import { describe, expect, it } from 'vitest'
import { recognitionPatch, type RecognizedField } from './recognitionPatch'

const RECOGNIZED = {
  title: 'Горный велосипед',
  category: 'Спорт и отдых',
  condition: 'good',
} as const

const edited = (...fields: RecognizedField[]) => new Set(fields)

describe('recognitionPatch — подстановка распознанного в форму', () => {
  it('заполняет все поля, если человек ничего не правил', () => {
    expect(recognitionPatch(RECOGNIZED, edited())).toEqual(RECOGNIZED)
  })

  it('не перетирает поле, которое человек заполнил сам', () => {
    const patch = recognitionPatch(RECOGNIZED, edited('title'))

    expect(patch.title).toBeUndefined()
    expect(patch).toEqual({ category: 'Спорт и отдых', condition: 'good' })
  })

  it('если правили всё, подставлять нечего', () => {
    expect(recognitionPatch(RECOGNIZED, edited('title', 'category', 'condition'))).toEqual({})
  })
})
