import { describe, expect, it } from 'vitest'
import { getBreadcrumbs, getSection } from './navigation'

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

describe('getBreadcrumbs', () => {
  it('на верхнем уровне крошек нет — там навигацию показывает меню кабинета', () => {
    expect(getBreadcrumbs('/')).toEqual([])
    expect(getBreadcrumbs('/exchange')).toEqual([])
    expect(getBreadcrumbs('/messages')).toEqual([])
  })

  it('переписка лежит под разделом «Сообщения»', () => {
    expect(getBreadcrumbs('/messages/c1/u2')).toEqual([
      { label: 'Авито' },
      { label: 'Сообщения', to: '/messages' },
      { label: 'Переписка' },
    ])
  })

  it('корень «Авито» некликабелен — главной Авито в демо нет', () => {
    expect(getBreadcrumbs('/items/new')[0]).toEqual({ label: 'Авито' })
  })

  it('новое объявление — путь назад в «Мои объявления»', () => {
    expect(getBreadcrumbs('/items/new')).toEqual([
      { label: 'Авито' },
      { label: 'Мои объявления', to: '/' },
      { label: 'Новое объявление' },
    ])
  })

  it('включение обмена — тоже внутри объявлений', () => {
    expect(getBreadcrumbs('/items/i1/barter')).toEqual([
      { label: 'Авито' },
      { label: 'Мои объявления', to: '/' },
      { label: 'Готов обменять' },
    ])
  })

  it('цепочка лежит под разделом «Обмен», а не под объявлениями', () => {
    expect(getBreadcrumbs('/exchange/c1')).toEqual([
      { label: 'Авито' },
      { label: 'Обмен', to: '/exchange' },
      { label: 'Цепочка' },
    ])
  })

  it('последняя крошка — текущая страница, без ссылки', () => {
    expect(getBreadcrumbs('/exchange/c1').at(-1)?.to).toBeUndefined()
  })

  it('незнакомый вложенный путь крошек не получает', () => {
    expect(getBreadcrumbs('/exchange/c1/extra')).toEqual([])
  })
})
