import { describe, expect, it } from 'vitest'
import type { Chain, ChainStatus, ParticipantStatus } from '../model/types'
import { cancelReason, countVariantsWithItem, isOpenOffer } from './offers'

/**
 * Цепочка из двоих: я отдаю вещь `myItem`, Костя — свою.
 * `myStatus === undefined` → меня в этой цепочке нет.
 */
const chain = (
  id: string,
  status: ChainStatus,
  myItem: string,
  myStatus?: ParticipantStatus,
): Chain => ({
  id,
  status,
  participants: [
    ...(myStatus
      ? [
          {
            userId: 'me',
            name: 'Вы',
            givesItem: { id: myItem, title: 'Горный велосипед' },
            status: myStatus,
            receiptConfirmed: false,
            isMe: true,
          },
        ]
      : []),
    {
      userId: 'u9',
      name: 'Костя',
      givesItem: { id: '91', title: 'Смартфон' },
      status: 'confirmed' as ParticipantStatus,
      receiptConfirmed: false,
    },
  ],
})

describe('isOpenOffer — предложение, ждущее моего ответа', () => {
  it('цепочка собирается, я не ответил → это предложение', () => {
    expect(isOpenOffer(chain('c1', 'formed', '1', 'pending'))).toBe(true)
  })

  it('я уже лайкнул → ответ дан, в блоке предложений делать нечего', () => {
    expect(isOpenOffer(chain('c1', 'formed', '1', 'confirmed'))).toBe(false)
  })

  it('я отказался → предложение закрыто', () => {
    expect(isOpenOffer(chain('c1', 'formed', '1', 'declined'))).toBe(false)
  })

  it('запущенный обмен предложением уже не является', () => {
    expect(isOpenOffer(chain('c1', 'active', '1', 'pending'))).toBe(false)
  })

  it('меня нет в цепочке → отвечать нечего', () => {
    expect(isOpenOffer(chain('c1', 'formed', '1'))).toBe(false)
  })
})

describe('countVariantsWithItem — конкуренция за мою вещь', () => {
  it('считает все живые варианты с этой вещью, включая уже лайкнутые', () => {
    const chains = [
      chain('c1', 'formed', '1', 'pending'),
      chain('c4', 'formed', '1', 'confirmed'),
      chain('c5', 'formed', '2', 'pending'),
    ]
    expect(countVariantsWithItem(chains, '1')).toBe(2)
    expect(countVariantsWithItem(chains, '2')).toBe(1)
  })

  it('вариант, от которого я отказался, за вещь больше не борется', () => {
    const chains = [chain('c1', 'formed', '1', 'declined'), chain('c4', 'formed', '1', 'pending')]
    expect(countVariantsWithItem(chains, '1')).toBe(1)
  })

  it('запущенные, завершённые и отменённые цепочки в счёт не идут', () => {
    const chains = [
      chain('c1', 'active', '1', 'confirmed'),
      chain('c2', 'completed', '1', 'confirmed'),
      chain('c3', 'cancelled', '1', 'pending'),
      chain('c4', 'dissolved', '1', 'declined'),
    ]
    expect(countVariantsWithItem(chains, '1')).toBe(0)
  })

  it('чужая вещь с тем же номером не считается моей', () => {
    expect(countVariantsWithItem([chain('c1', 'formed', '1', 'pending')], '91')).toBe(0)
  })

  it('вещи без вариантов — ноль, а не ошибка', () => {
    expect(countVariantsWithItem([], '1')).toBe(0)
  })
})

describe('cancelReason — почему вариант отменился', () => {
  it('ушла моя вещь — говорим о ней как о своей', () => {
    const cancelled: Chain = { ...chain('c1', 'cancelled', '1', 'pending'), cancelledItemId: '1' }
    expect(cancelReason(cancelled)).toBe('Ваша вещь «Горный велосипед» ушла в другой обмен')
  })

  it('ушла вещь другого участника — называем и вещь, и человека', () => {
    const cancelled: Chain = { ...chain('c1', 'cancelled', '1', 'pending'), cancelledItemId: '91' }
    expect(cancelReason(cancelled)).toBe('Вещь «Смартфон» участника Костя ушла в другой обмен')
  })

  it('бэкенд не назвал вещь → общая формулировка, но не пустота', () => {
    expect(cancelReason(chain('c1', 'cancelled', '1', 'pending'))).toBe(
      'Одна из вещей этого варианта ушла в другой обмен',
    )
  })
})
