import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import { isOpenOffer } from '../lib/offers'
import { findMe } from '../lib/participants'
import { getChain, getMyChains, respondToChain } from './chainApi'

const [DASHA, MARK] = PERSONAS

const lookAs = (personaId: string) => usePersonaStore.setState({ personaId })

beforeEach(() => {
  lookAs(DASHA.id)
})

describe('точка зрения персоны', () => {
  it('isMe достаётся выбранной персоне, а не лежит в данных', async () => {
    lookAs(MARK.id)
    const chain = await getChain('c1')

    expect(findMe(chain)?.userId).toBe(MARK.id)
    expect(chain.participants.filter((p) => p.isMe)).toHaveLength(1)
  })

  it('одна и та же цепочка глазами двух персон — разные «я»', async () => {
    const seenByDasha = await getChain('c1')
    lookAs(MARK.id)
    const seenByMark = await getChain('c1')

    expect(findMe(seenByDasha)?.userId).toBe(DASHA.id)
    expect(findMe(seenByMark)?.userId).toBe(MARK.id)
    expect(seenByDasha.participants.length).toBe(seenByMark.participants.length)
  })

  it('участник, за которого не смотрят, приходит без isMe', async () => {
    const chain = await getChain('c1')
    const mark = chain.participants.find((p) => p.userId === MARK.id)

    expect(mark?.isMe).toBe(undefined)
  })
})

describe('getMyChains', () => {
  it('возвращает только обмены, в которых участвует текущая персона', async () => {
    lookAs(MARK.id)
    const chains = await getMyChains()

    expect(chains.length).toBeGreaterThan(0)
    expect(chains.every((c) => c.participants.some((p) => p.userId === MARK.id))).toBe(true)
  })

  it('чужие обмены в кабинет не попадают', async () => {
    const dashaChains = await getMyChains()
    lookAs(MARK.id)
    const markChains = await getMyChains()

    const onlyDasha = dashaChains.filter((c) => !markChains.some((m) => m.id === c.id))
    expect(onlyDasha.length).toBeGreaterThan(0)
    expect(onlyDasha.every((c) => c.participants.every((p) => p.userId !== MARK.id))).toBe(true)
  })

  // Подбор параллельный: один вариант — это уже не список, и конкуренцию за вещь на нём не показать.
  it('предложений сразу несколько, и как минимум два держат одну и ту же вещь', async () => {
    const offers = (await getMyChains()).filter(isOpenOffer)
    const items = offers.map((c) => findMe(c)?.givesItem.id)

    expect(offers.length).toBeGreaterThan(1)
    expect(items.length - new Set(items).size).toBeGreaterThan(0)
  })
})

// Мутации идут последними: цепочки живут в модуле, и лайк меняет их для всех тестов ниже.
describe('ответ на предложение', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  /** Мок отвечает через `setTimeout` — прокручиваем таймеры вместо ожидания. */
  const run = async <T>(promise: Promise<T>, ms: number): Promise<T> => {
    await vi.advanceTimersByTimeAsync(ms)
    return promise
  }

  // Цепочки живут в модуле и между тестами не сбрасываются, поэтому порядок значим:
  // дизлайк распускает `c5`, а её же проверяет тест про отмену конкурентов.
  it('лайк последнего участника запускает обмен и отменяет конкурентов за ту же вещь', async () => {
    await run(respondToChain('c4', 'like'), 400)
    await vi.advanceTimersByTimeAsync(2500)

    const started = await run(getChain('c4'), 250)
    const rival = await run(getChain('c1'), 250)
    const untouched = await run(getChain('c5'), 250)

    expect(started.status).toBe('active')
    expect(started.participants.every((p) => p.status === 'confirmed')).toBe(true)

    // Велосипед Даши был и в c1, и в собравшейся c4 — c1 больше не собрать.
    expect(rival.status).toBe('cancelled')
    expect(rival.cancelledItemId).toBe('1')

    // Предложение с другой вещью отмена не задевает.
    expect(untouched.status).toBe('formed')
  })

  // Замену вышедшему сервис не ищет: вариант распадается целиком, а новый собирается заново —
  // при трёх участниках это дешевле, и часть тех же людей в него попадает снова.
  it('дизлайк распускает вариант целиком', async () => {
    await run(respondToChain('c5', 'dislike'), 400)
    const chain = await run(getChain('c5'), 250)

    expect(findMe(chain)?.status).toBe('declined')
    expect(chain.status).toBe('dissolved')
  })
})
