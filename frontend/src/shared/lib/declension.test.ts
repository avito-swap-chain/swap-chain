import { describe, expect, it } from 'vitest'
import { dative, genitive, instrumental } from './declension'

describe('dative', () => {
  it.each([
    ['Марк', 'Марку'],
    ['Иван', 'Ивану'],
    ['Даша', 'Даше'],
    ['Лена', 'Лене'],
    ['Рома', 'Роме'],
    ['Аня', 'Ане'],
    ['Илья', 'Илье'],
    ['Игорь', 'Игорю'],
    ['Андрей', 'Андрею'],
    ['Мария', 'Марии'],
  ])('%s → %s', (name, expected) => {
    expect(dative(name)).toBe(expected)
  })

  it('несклоняемое имя остаётся как есть', () => {
    expect(dative('Нино')).toBe('Нино')
  })
})

describe('genitive', () => {
  it.each([
    ['Марк', 'Марка'],
    ['Иван', 'Ивана'],
    ['Лена', 'Лены'],
    ['Рома', 'Ромы'],
    ['Даша', 'Даши'],
    ['Паша', 'Паши'],
    ['Ольга', 'Ольги'],
    ['Аня', 'Ани'],
    ['Катя', 'Кати'],
    ['Илья', 'Ильи'],
    ['Игорь', 'Игоря'],
    ['Андрей', 'Андрея'],
    ['Мария', 'Марии'],
  ])('%s → %s', (name, expected) => {
    expect(genitive(name)).toBe(expected)
  })

  it('несклоняемое имя остаётся как есть', () => {
    expect(genitive('Нино')).toBe('Нино')
  })
})

describe('instrumental — «обмен с …»', () => {
  it('склоняет имена по окончанию', () => {
    expect(instrumental('Марк')).toBe('Марком')
    expect(instrumental('Катя')).toBe('Катей')
    expect(instrumental('Даша')).toBe('Дашей')
    expect(instrumental('Лена')).toBe('Леной')
    expect(instrumental('Игорь')).toBe('Игорем')
  })

  it('незнакомое окончание оставляет как есть', () => {
    expect(instrumental('Ким')).toBe('Кимом')
    expect(instrumental('Ли')).toBe('Ли')
  })
})
