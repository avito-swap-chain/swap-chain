import { describe, expect, it } from 'vitest'
import { getSection } from './navigation'

describe('getSection', () => {
  it('корень — «Мои объявления»', () => {
    expect(getSection('/')).toBe('items')
  })

  it('список обменов — раздел «Обмен»', () => {
    expect(getSection('/exchange')).toBe('exchange')
  })

  it('открытая цепочка — тот же раздел «Обмен», что и список', () => {
    expect(getSection('/exchange/c1')).toBe('exchange')
  })

  it('создание объявления и включение обмена остаются в объявлениях', () => {
    expect(getSection('/items/new')).toBe('items')
    expect(getSection('/items/i1/barter')).toBe('items')
  })

  it('список переписок и открытый диалог — раздел «Сообщения»', () => {
    expect(getSection('/messages')).toBe('messages')
    expect(getSection('/messages/c1/u2')).toBe('messages')
  })

  it('уведомления — свой раздел, а не «Мои объявления»', () => {
    expect(getSection('/notifications')).toBe('notifications')
  })

  it('незнакомый роут не ломает подсветку', () => {
    expect(getSection('/whatever')).toBe('items')
  })

  it('похожий по началу путь не считается обменом', () => {
    expect(getSection('/exchanges')).toBe('items')
  })
})
