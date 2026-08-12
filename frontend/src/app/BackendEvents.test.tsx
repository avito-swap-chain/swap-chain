// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { chainKeys } from '@/entities/chain'
import { itemKeys } from '@/entities/item'
import { notificationKeys } from '@/entities/notification'
import { BackendEvents } from './BackendEvents'

/** Поток открывается только с подключённым бэкендом — этот режим и проверяем. */
vi.mock('@/shared/config/backend', () => ({ isBackendConnected: true }))

vi.mock('@/shared/model/session', () => ({
  sessionKeys: { current: () => ['session'] },
  getCurrentUser: () => Promise.resolve({ id: '1', name: 'Алиса' }),
  currentUserId: () => Promise.resolve(1),
}))

/**
 * `EventSource` в jsdom нет, а настоящий нам и не нужен: проверяем не разбор формата,
 * а что приход события помечает данные устаревшими. Подменяем его заглушкой, у которой
 * можно вызвать событие руками.
 */
class FakeEventSource {
  static last: FakeEventSource | undefined
  closed = false
  private readonly listeners = new Map<string, (event: MessageEvent) => void>()

  constructor() {
    FakeEventSource.last = this
  }

  addEventListener(name: string, handler: (event: MessageEvent) => void) {
    this.listeners.set(name, handler)
  }

  close() {
    this.closed = true
  }

  emit(name: string) {
    this.listeners.get(name)?.(new MessageEvent(name))
  }
}

const stale = (client: QueryClient, key: readonly unknown[]) =>
  client.getQueryState(key)?.isInvalidated === true

const listen = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

  // Открытые экраны, которые событие обязано обновить: список обменов и сама цепочка.
  // Ключи иерархические, поэтому инвалидация должна доставать до обоих сразу.
  client.setQueryData(chainKeys.my(), [])
  client.setQueryData(chainKeys.detail('21'), { id: '21' })
  client.setQueryData(itemKeys.my(), [])
  client.setQueryData(notificationKeys.all, [])

  render(
    <QueryClientProvider client={client}>
      <BackendEvents />
    </QueryClientProvider>,
  )

  return client
}

/** Подписка открывается после того, как загрузится сессия, — до неё потока нет. */
const stream = async () => {
  await waitFor(() => expect(FakeEventSource.last).toBeDefined())
  return FakeEventSource.last as FakeEventSource
}

describe('поток событий бэкенда', () => {
  beforeEach(() => {
    FakeEventSource.last = undefined
    vi.stubGlobal('EventSource', FakeEventSource)
  })

  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('на chain.updated помечает устаревшими и список обменов, и саму цепочку', async () => {
    const client = listen()
    ;(await stream()).emit('chain.updated')

    await waitFor(() => {
      expect(stale(client, chainKeys.my())).toBe(true)
      expect(stale(client, chainKeys.detail('21'))).toBe(true)
    })
  })

  /**
   * Событие — только сигнал: одно и то же состояние бэкенд шлёт по нескольку раз
   * (переход доставки задевает всех участников цепочки). Повтор обязан быть безобидным.
   */
  it('повторное событие ничего не ломает', async () => {
    const client = listen()
    const source = await stream()

    source.emit('chain.updated')
    source.emit('chain.updated')
    source.emit('chain.updated')

    await waitFor(() => expect(stale(client, chainKeys.detail('21'))).toBe(true))
    expect(client.getQueryData(chainKeys.detail('21'))).toEqual({ id: '21' })
  })

  /**
   * Пропущенное за время обрыва сервер не переотправляет, поэтому после подключения
   * состояние перечитывается целиком — иначе экран останется таким, каким был до разрыва.
   */
  it('после stream.connected перечитывает всё состояние', async () => {
    const client = listen()
    ;(await stream()).emit('stream.connected')

    await waitFor(() => {
      expect(stale(client, chainKeys.my())).toBe(true)
      expect(stale(client, itemKeys.my())).toBe(true)
      expect(stale(client, notificationKeys.all)).toBe(true)
    })
  })
})
