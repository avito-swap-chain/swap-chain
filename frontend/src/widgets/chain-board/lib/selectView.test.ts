import { describe, expect, it } from 'vitest'
import type { Chain, ChainParticipant, ChainStatus, ParticipantStatus } from '@/entities/chain'
import { selectView, type ChainView } from './selectView'

const other = (userId: string, status: ParticipantStatus = 'confirmed'): ChainParticipant => ({
  userId,
  name: `Участник ${userId}`,
  givesItem: { id: `item-${userId}`, title: `Вещь ${userId}` },
  status,
  receiptConfirmed: false,
})

/** Цепочка из троих, где я — второй. `myStatus === undefined` → меня в цепочке нет. */
const chainWithMe = (status: ChainStatus, myStatus?: ParticipantStatus): Chain => ({
  id: 'c1',
  status,
  participants: [
    other('u1'),
    ...(myStatus
      ? [
          {
            userId: 'me',
            name: 'Вы',
            givesItem: { id: 'item-me', title: 'Велосипед' },
            status: myStatus,
            receiptConfirmed: false,
            isMe: true,
          } satisfies ChainParticipant,
        ]
      : []),
    other('u3'),
  ],
})

describe('selectView — цепочка formed, вид зависит от моего ответа', () => {
  it('я ещё не ответил → показываем предложение обмена', () => {
    expect(selectView(chainWithMe('formed', 'pending'))).toBe('offer')
  })

  it('я подтвердил → показываем ожидание остальных', () => {
    expect(selectView(chainWithMe('formed', 'confirmed'))).toBe('waiting')
  })

  // Я вышел, но цепочка жива: бэкенд ищет мне замену (Миша ↔ Витя ↔ X).
  // Показывать отказавшемуся предложение с кнопкой «подтвердить» нельзя — у состояния свой вид.
  it('я отказался, но цепочка ещё formed → вид «вы вышли»', () => {
    expect(selectView(chainWithMe('formed', 'declined'))).toBe('declined')
  })

  // Сам selectView этот случай не различает — «не участвую» отсекается выше, в ChainBoard.
  it('меня нет в цепочке → предложение', () => {
    expect(selectView(chainWithMe('formed'))).toBe('offer')
  })

  it('цепочка без участников → предложение', () => {
    expect(selectView({ id: 'c1', status: 'formed', participants: [] })).toBe('offer')
  })
})

describe('selectView — терминальные статусы перекрывают статус участника', () => {
  const myStatuses: ParticipantStatus[] = ['pending', 'confirmed', 'declined']

  const cases: { status: ChainStatus; view: ChainView }[] = [
    { status: 'active', view: 'handoff' },
    { status: 'completed', view: 'completed' },
    { status: 'dissolved', view: 'dissolved' },
    // Вещь ушла в другую цепочку: отвечать на предложение уже поздно, что бы я ни ответил раньше.
    { status: 'cancelled', view: 'cancelled' },
  ]

  for (const { status, view } of cases) {
    it.each(myStatuses)(`цепочка ${status} + мой статус %s → ${view}`, (myStatus) => {
      expect(selectView(chainWithMe(status, myStatus))).toBe(view)
    })

    it(`цепочка ${status} без меня → ${view}`, () => {
      expect(selectView(chainWithMe(status))).toBe(view)
    })
  }
})

describe('selectView — свойства машины состояний', () => {
  it('покрыты все статусы цепочки, ни один не проваливается в undefined', () => {
    const statuses: ChainStatus[] = ['formed', 'active', 'completed', 'dissolved', 'cancelled']
    const views = statuses.map((status) => selectView(chainWithMe(status, 'confirmed')))
    expect(views).toEqual(['waiting', 'handoff', 'completed', 'dissolved', 'cancelled'])
  })

  it('статус участника влияет на вид только при formed', () => {
    const terminal: ChainStatus[] = ['active', 'completed', 'dissolved', 'cancelled']
    for (const status of terminal) {
      const withPending = selectView(chainWithMe(status, 'pending'))
      const withConfirmed = selectView(chainWithMe(status, 'confirmed'))
      expect(withPending).toBe(withConfirmed)
    }
    expect(selectView(chainWithMe('formed', 'pending'))).not.toBe(
      selectView(chainWithMe('formed', 'confirmed')),
    )
  })

  it('не мутирует переданную цепочку', () => {
    const chain = chainWithMe('formed', 'pending')
    const snapshot = structuredClone(chain)
    selectView(chain)
    expect(chain).toEqual(snapshot)
  })
})
