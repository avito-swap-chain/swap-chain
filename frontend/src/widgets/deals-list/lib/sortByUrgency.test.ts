import { describe, expect, it } from 'vitest'
import type { Chain, ChainParticipant, ChainStatus, ParticipantStatus } from '@/entities/chain'
import { sortByUrgency } from './sortByUrgency'

/**
 * Цепочка из двоих: я и ещё один участник. `myStatus === undefined` → меня в ней нет.
 * `myReceipt` — отметил ли я получение вещи, важно на стадии передачи.
 */
const chain = (
  id: string,
  status: ChainStatus,
  myStatus?: ParticipantStatus,
  myReceipt = false,
): Chain => ({
  id,
  status,
  participants: [
    {
      userId: 'u1',
      name: 'Марк',
      givesItem: { id: 'item-u1', title: 'Наушники' },
      status: 'confirmed',
      receiptConfirmed: false,
    },
    ...(myStatus
      ? [
          {
            userId: 'me',
            name: 'Вы',
            givesItem: { id: 'item-me', title: 'Велосипед' },
            status: myStatus,
            receiptConfirmed: myReceipt,
            isMe: true,
          } satisfies ChainParticipant,
        ]
      : []),
  ],
})

const ids = (chains: Chain[]) => chains.map((c) => c.id)

describe('sortByUrgency', () => {
  it('цепочка, ждущая моего ответа, поднимается в самый верх', () => {
    const input = [
      chain('completed', 'completed', 'confirmed'),
      chain('urgent', 'formed', 'pending'),
      chain('active', 'active', 'confirmed', true),
    ]
    expect(ids(sortByUrgency(input))).toEqual(['urgent', 'active', 'completed'])
  })

  it('порядок статусов: formed → active → dissolved → cancelled → completed', () => {
    const input = [
      chain('completed', 'completed', 'confirmed'),
      chain('cancelled', 'cancelled', 'pending'),
      chain('dissolved', 'dissolved', 'declined'),
      chain('active', 'active', 'confirmed', true),
      chain('formed', 'formed', 'confirmed'),
    ]
    expect(ids(sortByUrgency(input))).toEqual([
      'formed',
      'active',
      'dissolved',
      'cancelled',
      'completed',
    ])
  })

  it('срочная цепочка обгоняет даже formed, где я уже подтвердил', () => {
    const input = [chain('waiting', 'formed', 'confirmed'), chain('urgent', 'formed', 'pending')]
    expect(ids(sortByUrgency(input))).toEqual(['urgent', 'waiting'])
  })

  it('мой отказ снимает срочность: цепочка падает вниз к остальным formed', () => {
    const input = [chain('declined', 'formed', 'declined'), chain('urgent', 'formed', 'pending')]
    expect(ids(sortByUrgency(input))).toEqual(['urgent', 'declined'])
  })

  it('цепочка без меня никогда не срочная', () => {
    const input = [chain('stranger', 'formed'), chain('urgent', 'formed', 'pending')]
    expect(ids(sortByUrgency(input))).toEqual(['urgent', 'stranger'])
  })

  it('в запущенной цепочке срочность решает отметка получения, а не статус ответа', () => {
    const input = [
      chain('active', 'active', 'pending', true),
      chain('formed', 'formed', 'confirmed'),
    ]
    expect(ids(sortByUrgency(input))).toEqual(['formed', 'active'])
  })

  it('запущенная цепочка срочная, пока я не отметил получение', () => {
    const input = [chain('formed', 'formed', 'confirmed'), chain('handoff', 'active', 'confirmed')]
    expect(ids(sortByUrgency(input))).toEqual(['handoff', 'formed'])
  })

  it('при равной срочности сохраняет исходный порядок (сортировка стабильна)', () => {
    const input = [
      chain('a', 'active', 'confirmed', true),
      chain('b', 'active', 'confirmed', true),
      chain('c', 'active', 'confirmed', true),
    ]
    expect(ids(sortByUrgency(input))).toEqual(['a', 'b', 'c'])
  })

  it('несколько срочных цепочек сохраняют исходный порядок между собой', () => {
    const input = [
      chain('u1', 'formed', 'pending'),
      chain('done', 'completed', 'confirmed'),
      chain('u2', 'formed', 'pending'),
    ]
    expect(ids(sortByUrgency(input))).toEqual(['u1', 'u2', 'done'])
  })

  it('не мутирует исходный массив и возвращает новый', () => {
    const input = [
      chain('completed', 'completed', 'confirmed'),
      chain('urgent', 'formed', 'pending'),
    ]
    const result = sortByUrgency(input)
    expect(result).not.toBe(input)
    expect(ids(input)).toEqual(['completed', 'urgent'])
  })

  it('пустой список остаётся пустым', () => {
    expect(sortByUrgency([])).toEqual([])
  })

  it('один элемент возвращается как есть', () => {
    expect(ids(sortByUrgency([chain('only', 'dissolved', 'declined')]))).toEqual(['only'])
  })
})
