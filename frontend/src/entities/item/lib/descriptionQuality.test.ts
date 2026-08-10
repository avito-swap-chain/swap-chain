import { describe, expect, it } from 'vitest'
import { descriptionQuality } from './descriptionQuality'

describe('descriptionQuality — годность описания для подбора', () => {
  it('короткое описание подбору почти ничего не даёт', () => {
    expect(descriptionQuality('Велосипед')).toBe('short')
    expect(descriptionQuality('')).toBe('short')
  })

  it('среднее описание уже работает, но не в полную силу', () => {
    expect(descriptionQuality('Горный велосипед, рама 19 дюймов')).toBe('fair')
  })

  it('подробное описание — то, что нужно подбору', () => {
    expect(
      descriptionQuality('Горный велосипед, рама 19 дюймов, катался два сезона, есть насос'),
    ).toBe('good')
  })

  it('границы порогов: 20 символов уже не короткое, 45 уже хорошее', () => {
    expect(descriptionQuality('я'.repeat(19))).toBe('short')
    expect(descriptionQuality('я'.repeat(20))).toBe('fair')
    expect(descriptionQuality('я'.repeat(44))).toBe('fair')
    expect(descriptionQuality('я'.repeat(45))).toBe('good')
  })

  it('пробелы по краям не считаются за содержание', () => {
    expect(descriptionQuality(`   ${'я'.repeat(19)}   `)).toBe('short')
  })
})
