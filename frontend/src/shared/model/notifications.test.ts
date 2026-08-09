import { beforeEach, describe, expect, it } from 'vitest'
import {
  countUnreadNotifications,
  listNotifications,
  markNotificationsRead,
  notify,
  resetNotifications,
} from './notifications'
import { PERSONAS, usePersonaStore } from './persona'

const [DASHA, MARK] = PERSONAS

const event = (title: string) => ({ kind: 'chain' as const, title, text: '…', to: '/exchange' })

describe('журнал уведомлений', () => {
  beforeEach(() => {
    resetNotifications()
    usePersonaStore.setState({ personaId: DASHA.id })
  })

  it('пока ничего не произошло, список пуст', () => {
    expect(listNotifications()).toEqual([])
    expect(countUnreadNotifications()).toBe(0)
  })

  it('записанное событие попадает в список непрочитанным', () => {
    notify(event('Цепочка собралась'))

    const [first] = listNotifications()
    expect(first).toMatchObject({ title: 'Цепочка собралась', read: false })
    expect(countUnreadNotifications()).toBe(1)
  })

  it('свежие события идут первыми', () => {
    notify(event('Первое'))
    notify(event('Второе'))

    expect(listNotifications().map((n) => n.title)).toEqual(['Второе', 'Первое'])
  })

  it('открытие списка гасит счётчик, но записи остаются', () => {
    notify(event('Цепочка собралась'))
    markNotificationsRead()

    expect(countUnreadNotifications()).toBe(0)
    expect(listNotifications()).toHaveLength(1)
    expect(listNotifications()[0].read).toBe(true)
  })

  it('уведомление достаётся той персоне, которая была «я» в момент события', () => {
    notify(event('Цепочка собралась'))

    usePersonaStore.setState({ personaId: MARK.id })
    expect(listNotifications()).toEqual([])
    expect(countUnreadNotifications()).toBe(0)

    usePersonaStore.setState({ personaId: DASHA.id })
    expect(listNotifications()).toHaveLength(1)
  })
})
