// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/shared/api/fetcher'
import { LoginPage } from './LoginPage'

const login = vi.fn()
const register = vi.fn()

vi.mock('@/shared/model/session', () => ({
  DEMO_USERS: [{ name: 'Алиса', phone: '+79001000001' }],
  login: (phone: string) => login(phone),
  register: (name: string, phone: string) => register(name, phone),
}))

/** Незнакомый бэкенду номер: именно так выглядит развилка на регистрацию. */
const unknownPhone = () =>
  Promise.reject(new ApiError(404, 'no user is registered with this phone'))

const show = () => {
  // Повторы выключены: тест проверяет реакцию на ответ, а не терпение клиента.
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })

  return render(
    <QueryClientProvider client={client}>
      <LoginPage />
    </QueryClientProvider>,
  )
}

describe('экран входа', () => {
  beforeEach(() => {
    login.mockReset()
    register.mockReset()
  })

  // Разметку убираем руками: авто-очистка включается только с `globals`, а их в проекте нет —
  // без этого следующий тест видит поля предыдущего и не может выбрать из двух одинаковых.
  afterEach(cleanup)

  it('незнакомый номер — не ошибка, а развилка: форма просит имя', async () => {
    login.mockImplementation(unknownPhone)
    show()

    await userEvent.type(screen.getByLabelText('Телефон'), '+79005550101')
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }))

    expect(await screen.findByLabelText('Имя')).toBeDefined()
    expect(screen.getByRole('button', { name: 'Зарегистрироваться' })).toBeDefined()
  })

  /**
   * Регрессия: обработчик читал признак «новый пользователь» из замыкания, и после неудачного
   * входа он залипал — кнопка демо-пользователя уходила в регистрацию вместо входа.
   */
  it('после неудачного входа демо-кнопка всё равно входит, а не регистрирует', async () => {
    login.mockImplementation(unknownPhone)
    show()

    await userEvent.type(screen.getByLabelText('Телефон'), '+79005550101')
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }))
    await screen.findByLabelText('Имя')

    login.mockResolvedValue({ id: '1', name: 'Алиса' })
    await userEvent.click(screen.getByRole('button', { name: 'Алиса' }))

    expect(login).toHaveBeenLastCalledWith('+79001000001')
    expect(register).not.toHaveBeenCalled()
  })

  it('кривой номер до бэкенда не доходит — он отвечает на такое дампом схемы', async () => {
    show()

    await userEvent.type(screen.getByLabelText('Телефон'), '123')
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }))

    expect(await screen.findByRole('alert')).toBeDefined()
    expect(login).not.toHaveBeenCalled()
  })

  it('номер приводится к одному виду: `8…` и `+7…` — один и тот же человек', async () => {
    login.mockResolvedValue({ id: '1', name: 'Кто-то' })
    show()

    await userEvent.type(screen.getByLabelText('Телефон'), '8 900 555 01 01')
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }))

    expect(login).toHaveBeenCalledWith('+79005550101')
  })
})
