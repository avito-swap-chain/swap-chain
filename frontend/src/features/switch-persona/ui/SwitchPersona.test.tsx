// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SwitchPersona } from './SwitchPersona'

/** С бэкендом переключатель работает поверх настоящих сессий — этот режим и проверяем. */
vi.mock('@/shared/config/backend', () => ({ isBackendConnected: true }))

/**
 * Сейчас смотрим глазами Алисы, а своим номером человек входил раньше.
 * Данные лежат внутри фабрики: `vi.mock` поднимается выше объявлений модуля.
 */
vi.mock('@/shared/model/session', () => ({
  DEMO_USERS: [
    { name: 'Алиса', phone: '+79001000001' },
    { name: 'ПВЗ Администратор', phone: '+79009999999', isAdmin: true },
  ],
  sessionKeys: { current: () => ['session'] },
  getCurrentUser: () => Promise.resolve({ id: '1', name: 'Алиса', phone: '+79001000001' }),
  knownAccounts: () => [
    { name: 'Игорь', phone: '+79007771234' },
    { name: 'Алиса', phone: '+79001000001' },
  ],
  login: vi.fn(),
  logout: vi.fn(),
}))

const show = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

  // Меню аккаунта ведёт в разделы кабинета ссылками, поэтому ему нужен роутер.
  return render(
    <MemoryRouter>
      <QueryClientProvider client={client}>
        <SwitchPersona />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}

describe('переключатель аккаунтов', () => {
  afterEach(cleanup)

  /**
   * Замечание ментора: он зарегистрировался своим номером, переключился на демо-аккаунт —
   * и собственный профиль пропал из списка, вернуться в него было нечем.
   */
  it('свой аккаунт остаётся в списке, даже когда смотрим чужими глазами', async () => {
    show()

    const options = await screen.findAllByRole('option')
    const names = options.map((option) => option.textContent)

    expect(names).toContain('Игорь')
    expect(names).toContain('Алиса')
  })

  it('демо-пользователи не задваиваются с теми, под кем уже входили', async () => {
    show()

    const names = (await screen.findAllByRole('option')).map((option) => option.textContent)

    expect(names.filter((name) => name === 'Алиса')).toHaveLength(1)
  })

  it('сотрудник ПВЗ помечен короной — иначе не видно, под кем открывать админку', async () => {
    show()

    const names = (await screen.findAllByRole('option')).map((option) => option.textContent)

    expect(names).toContain('ПВЗ Администратор 👑')
  })
})
