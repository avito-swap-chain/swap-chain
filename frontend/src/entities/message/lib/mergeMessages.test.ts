import { describe, expect, it } from 'vitest'
import type { Message } from '../model/types'
import { mergeMessages } from './mergeMessages'

const message = (id: string, author: Message['author'] = 'them'): Message => ({
  id,
  author,
  text: `реплика ${id}`,
  createdAt: '2026-08-10T10:00:00.000Z',
})

describe('mergeMessages — долив реплик из long-poll', () => {
  it('свежие реплики встают в конец ленты', () => {
    const merged = mergeMessages([message('1')], [message('2'), message('3')])

    expect(merged.map((m) => m.id)).toEqual(['1', '2', '3'])
  })

  it('сообщение, показанное сразу после отправки, не задваивается опросом', () => {
    const mine = message('2', 'me')
    const merged = mergeMessages([message('1'), mine], [mine, message('3')])

    expect(merged.map((m) => m.id)).toEqual(['1', '2', '3'])
  })

  it('пустой ответ long-poll оставляет прежний массив — перерисовывать нечего', () => {
    const known = [message('1')]

    expect(mergeMessages(known, [])).toBe(known)
    expect(mergeMessages(known, [message('1')])).toBe(known)
  })
})
