import { describe, expect, it } from 'vitest'
import type { Chain, ChainParticipant, ChainStatus, ParticipantStatus } from '../model/types'
import {
  confirmReceiptFor,
  countConfirmed,
  countReceipts,
  displayName,
  findDecliner,
  findMe,
  findNeighbours,
  isNeighbour,
  needsMyAction,
} from './participants'

const participant = (
  userId: string,
  status: ParticipantStatus,
  isMe?: boolean,
): ChainParticipant => ({
  userId,
  name: `Участник ${userId}`,
  givesItem: { id: `item-${userId}`, title: `Вещь ${userId}` },
  status,
  receiptConfirmed: false,
  ...(isMe ? { isMe: true } : {}),
})

/** Участник на стадии передачи: участие подтверждено всеми, вопрос только в получении вещи. */
const receiver = (userId: string, receiptConfirmed: boolean, isMe?: boolean): ChainParticipant => ({
  ...participant(userId, 'confirmed', isMe),
  receiptConfirmed,
})

const chain = (participants: ChainParticipant[], status: ChainStatus = 'formed'): Chain => ({
  id: 'c1',
  status,
  participants,
})

describe('findMe', () => {
  it('находит участника с флагом isMe', () => {
    const me = participant('me', 'pending', true)
    expect(findMe(chain([participant('u1', 'confirmed'), me]))).toBe(me)
  })

  it('возвращает undefined, если текущего пользователя нет в цепочке', () => {
    expect(findMe(chain([participant('u1', 'confirmed'), participant('u2', 'pending')]))).toBe(
      undefined,
    )
  })

  it('возвращает undefined для пустого списка участников', () => {
    expect(findMe(chain([]))).toBe(undefined)
  })

  it('при нескольких isMe (некорректные данные) берёт первого', () => {
    const first = participant('me-1', 'pending', true)
    const second = participant('me-2', 'confirmed', true)
    expect(findMe(chain([first, second]))).toBe(first)
  })
})

describe('displayName', () => {
  it('себя пользователь читает как «Вы», а не по имени', () => {
    expect(displayName(participant('me', 'pending', true))).toBe('Вы')
  })

  it('остальных — по имени из данных', () => {
    expect(displayName(participant('u1', 'confirmed'))).toBe('Участник u1')
  })
})

describe('needsMyAction', () => {
  it('цепочка formed и я ещё не ответил → ход за мной', () => {
    expect(needsMyAction(chain([participant('me', 'pending', true)], 'formed'))).toBe(true)
  })

  it('цепочка formed, но я уже подтвердил → жду остальных, действий не нужно', () => {
    expect(needsMyAction(chain([participant('me', 'confirmed', true)], 'formed'))).toBe(false)
  })

  it('цепочка formed, но я уже отказался → действий не нужно', () => {
    expect(needsMyAction(chain([participant('me', 'declined', true)], 'formed'))).toBe(false)
  })

  it.each<ChainStatus>(['completed', 'dissolved'])(
    'статус %s + мой pending → не срочно, отвечать уже поздно',
    (status) => {
      expect(needsMyAction(chain([participant('me', 'pending', true)], status))).toBe(false)
    },
  )

  it('цепочка active и я ещё не отметил получение → ход за мной', () => {
    expect(needsMyAction(chain([receiver('me', false, true)], 'active'))).toBe(true)
  })

  it('цепочка active, но получение я уже отметил → жду остальных', () => {
    expect(needsMyAction(chain([receiver('me', true, true)], 'active'))).toBe(false)
  })

  it('на стадии передачи важна отметка получения, а не статус ответа', () => {
    expect(needsMyAction(chain([participant('me', 'pending', true)], 'active'))).toBe(true)
  })

  it('цепочка закрыта → отмечать больше нечего', () => {
    expect(needsMyAction(chain([receiver('me', true, true)], 'completed'))).toBe(false)
  })

  it('меня нет в цепочке → ход не за мной', () => {
    expect(needsMyAction(chain([participant('u1', 'pending')], 'formed'))).toBe(false)
  })

  it('меня нет в цепочке на стадии передачи → ход не за мной', () => {
    expect(needsMyAction(chain([receiver('u1', false)], 'active'))).toBe(false)
  })
})

describe('countConfirmed', () => {
  it('считает только подтвердивших, игнорируя pending и declined', () => {
    const c = chain([
      participant('u1', 'confirmed'),
      participant('u2', 'pending'),
      participant('u3', 'declined'),
      participant('me', 'confirmed', true),
    ])
    expect(countConfirmed(c)).toBe(2)
  })

  it('никто не подтвердил → 0', () => {
    expect(countConfirmed(chain([participant('u1', 'pending')]))).toBe(0)
  })

  it('пустая цепочка → 0', () => {
    expect(countConfirmed(chain([]))).toBe(0)
  })

  it('все подтвердили → длина списка', () => {
    const c = chain([participant('u1', 'confirmed'), participant('u2', 'confirmed')])
    expect(countConfirmed(c)).toBe(2)
  })
})

describe('countReceipts', () => {
  it('считает участников, отметивших получение', () => {
    const c = chain([receiver('u1', true), receiver('u2', false), receiver('me', true, true)])
    expect(countReceipts(c)).toBe(2)
  })

  it('никто не отметил → 0', () => {
    expect(countReceipts(chain([receiver('u1', false)]))).toBe(0)
  })

  it('пустая цепочка → 0', () => {
    expect(countReceipts(chain([]))).toBe(0)
  })
})

describe('confirmReceiptFor', () => {
  const handoff = (...participants: ChainParticipant[]) => chain(participants, 'active')

  it('отмечает только указанного участника, остальных не трогает', () => {
    const result = confirmReceiptFor(
      handoff(receiver('me', false, true), receiver('u2', false)),
      'me',
    )

    expect(result.participants.map((p) => p.receiptConfirmed)).toEqual([true, false])
  })

  it('подтверждение одного участника не закрывает цепочку', () => {
    const result = confirmReceiptFor(
      handoff(receiver('me', false, true), receiver('u2', false), receiver('u3', false)),
      'me',
    )

    expect(result.status).toBe('active')
  })

  it('цепочка остаётся открытой, пока не отметил хотя бы один участник', () => {
    const result = confirmReceiptFor(
      handoff(receiver('me', false, true), receiver('u2', true), receiver('u3', false)),
      'me',
    )

    expect(result.status).toBe('active')
    expect(countReceipts(result)).toBe(2)
  })

  it('подтверждение последнего участника закрывает цепочку', () => {
    const result = confirmReceiptFor(
      handoff(receiver('me', false, true), receiver('u2', true), receiver('u3', true)),
      'me',
    )

    expect(result.status).toBe('completed')
    expect(countReceipts(result)).toBe(3)
  })

  it('повторная отметка того же участника ничего не закрывает', () => {
    const once = confirmReceiptFor(
      handoff(receiver('me', false, true), receiver('u2', false)),
      'me',
    )
    const twice = confirmReceiptFor(once, 'me')

    expect(twice.status).toBe('active')
    expect(countReceipts(twice)).toBe(1)
  })

  it('участника нет в цепочке → ничего не меняется', () => {
    const before = handoff(receiver('me', false, true), receiver('u2', false))
    const result = confirmReceiptFor(before, 'stranger')

    expect(result.status).toBe('active')
    expect(countReceipts(result)).toBe(0)
  })

  it('не мутирует исходную цепочку', () => {
    const before = handoff(receiver('me', false, true), receiver('u2', true))
    const snapshot = structuredClone(before)

    confirmReceiptFor(before, 'me')

    expect(before).toEqual(snapshot)
  })
})

describe('findDecliner', () => {
  it('находит отказавшегося участника', () => {
    const decliner = participant('u2', 'declined')
    expect(findDecliner(chain([participant('u1', 'confirmed'), decliner]))).toBe(decliner)
  })

  it('находит отказ самого пользователя', () => {
    const me = participant('me', 'declined', true)
    expect(findDecliner(chain([participant('u1', 'confirmed'), me]))).toBe(me)
  })

  it('никто не отказался → undefined', () => {
    const c = chain([participant('u1', 'confirmed'), participant('u2', 'pending')])
    expect(findDecliner(c)).toBe(undefined)
  })

  it('при нескольких отказах возвращает первого по порядку обхода', () => {
    const first = participant('u1', 'declined')
    const second = participant('u2', 'declined')
    expect(findDecliner(chain([first, second]))).toBe(first)
  })
})

describe('findNeighbours', () => {
  it('я в середине круга: отдаю следующему, получаю от предыдущего', () => {
    const c = chain([
      participant('u1', 'confirmed'),
      participant('me', 'pending', true),
      participant('u3', 'confirmed'),
    ])
    expect(findNeighbours(c)).toEqual({
      receiver: c.participants[2],
      giver: c.participants[0],
    })
  })

  it('я первый в списке: получаю от последнего — круг замыкается', () => {
    const c = chain([
      participant('me', 'pending', true),
      participant('u2', 'confirmed'),
      participant('u3', 'confirmed'),
    ])
    expect(findNeighbours(c)).toEqual({
      receiver: c.participants[1],
      giver: c.participants[2],
    })
  })

  it('я последний в списке: отдаю первому — круг замыкается', () => {
    const c = chain([
      participant('u1', 'confirmed'),
      participant('u2', 'confirmed'),
      participant('me', 'pending', true),
    ])
    expect(findNeighbours(c)).toEqual({
      receiver: c.participants[0],
      giver: c.participants[1],
    })
  })

  it('цепочка из двоих: и отдаю, и получаю от одного и того же человека', () => {
    const c = chain([participant('me', 'pending', true), participant('u2', 'confirmed')])
    const neighbours = findNeighbours(c)
    expect(neighbours?.receiver).toBe(c.participants[1])
    expect(neighbours?.giver).toBe(c.participants[1])
  })

  it('вырожденная цепочка из одного меня: обоими соседями оказываюсь я сам', () => {
    const c = chain([participant('me', 'pending', true)])
    const neighbours = findNeighbours(c)
    expect(neighbours?.receiver).toBe(c.participants[0])
    expect(neighbours?.giver).toBe(c.participants[0])
  })

  it('меня нет в цепочке → undefined', () => {
    expect(findNeighbours(chain([participant('u1', 'confirmed')]))).toBe(undefined)
  })

  it('пустая цепочка → undefined', () => {
    expect(findNeighbours(chain([]))).toBe(undefined)
  })

  it('соседи не зависят от статусов участников', () => {
    const c = chain(
      [
        participant('u1', 'declined'),
        participant('me', 'confirmed', true),
        participant('u3', 'pending'),
      ],
      'dissolved',
    )
    expect(findNeighbours(c)).toEqual({
      receiver: c.participants[2],
      giver: c.participants[0],
    })
  })
})

describe('isNeighbour', () => {
  const c = chain([
    participant('me', 'pending', true),
    participant('u2', 'confirmed'),
    participant('u3', 'confirmed'),
    participant('u4', 'confirmed'),
  ])
  const neighbours = findNeighbours(c)!

  it('тот, кому отдаю вещь, — сосед', () => {
    expect(isNeighbour(c.participants[1], neighbours)).toBe(true)
  })

  it('тот, от кого получаю вещь, — сосед', () => {
    expect(isNeighbour(c.participants[3], neighbours)).toBe(true)
  })

  it('участник на другой стороне круга соседом не считается', () => {
    expect(isNeighbour(c.participants[2], neighbours)).toBe(false)
  })

  it('сам пользователь себе не сосед', () => {
    expect(isNeighbour(c.participants[0], neighbours)).toBe(false)
  })

  it('в цепочке из троих соседями оказываются оба участника', () => {
    const three = chain([
      participant('me', 'pending', true),
      participant('u2', 'confirmed'),
      participant('u3', 'confirmed'),
    ])
    const both = findNeighbours(three)!

    expect(three.participants.filter((p) => isNeighbour(p, both))).toHaveLength(2)
  })
})
