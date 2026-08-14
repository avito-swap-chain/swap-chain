import { useState, type SubmitEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { describeError } from '@/shared/api/describeError'
import { ApiError } from '@/shared/api/fetcher'
import { normalizePhone } from '@/shared/lib'
import { accountLabel, DEMO_USERS, login, register } from '@/shared/model/session'
import { Button, Input } from '@/shared/ui'

/**
 * Вход в кабинет. Пароля в контракте нет: бэкенд выдаёт сессию в обмен на телефон,
 * поэтому форма из одного поля, а регистрация — тот же телефон плюс имя.
 *
 * Незнакомый номер не ошибка, а развилка: бэкенд отвечает 404, и мы дорисовываем поле
 * имени вместо того, чтобы отправлять человека на отдельный экран регистрации.
 */
export function LoginPage() {
  const [phone, setPhone] = useState('')
  const [name, setName] = useState('')
  const [isNewUser, setIsNewUser] = useState(false)
  const [isPhoneBroken, setIsPhoneBroken] = useState(false)
  const queryClient = useQueryClient()

  /**
   * Что делаем — решает вызывающий и передаёт вместе с данными. Читать `isNewUser` внутри
   * `mutationFn` нельзя: состояние React обновляется асинхронно, и кнопка демо-пользователя,
   * которая сбрасывает флаг прямо перед отправкой, уходила бы в регистрацию по старому
   * значению. Имя есть — регистрируем, нет — входим.
   */
  const enter = useMutation({
    mutationFn: ({ phone, name }: { phone: string; name?: string }) =>
      name === undefined ? login(phone) : register(name, phone),
    onSuccess: () => queryClient.resetQueries(),
    onError: (error) => {
      // 404 — такого телефона у бэкенда нет. Значит человек здесь впервые.
      if (error instanceof ApiError && error.status === 404) setIsNewUser(true)
    },
  })

  /** Другой номер — снова неизвестно, знаком ли он бэкенду: поле имени убираем до ответа. */
  const editPhone = (value: string) => {
    setPhone(value)
    setIsNewUser(false)
    setIsPhoneBroken(false)
  }

  /**
   * Номер проверяем до отправки. Бэкенд знает про телефон только длину и на коротком отвечает
   * дампом схемы валидатора — этот текст попадал прямо в форму, и на нём споткнулась проверка.
   * Заодно человек не заводит три аккаунта на один номер, набрав его через `8`, `+7` и без кода.
   */
  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()

    const normalized = normalizePhone(phone)
    if (!normalized) {
      setIsPhoneBroken(true)
      return
    }

    enter.mutate({ phone: normalized, name: isNewUser ? name.trim() : undefined })
  }

  // Ответ бэкенда пересказываем своими словами: в его `message` лежит текст для разработчика.
  // 404 сюда не попадает — незнакомый номер это не ошибка, а развилка на регистрацию.
  // Сбой сети проверкой на `ApiError` не ловился, и форма молчала при лежащем бэкенде.
  const failed = enter.error && !(enter.error instanceof ApiError && enter.error.status === 404)

  const message = isPhoneBroken
    ? 'Проверьте номер — нужен российский, например +7 900 100-00-01'
    : failed
      ? describeError(enter.error)
      : undefined

  return (
    <div className="flex min-h-svh items-center justify-center bg-page px-4">
      <form
        onSubmit={submit}
        className="flex w-full max-w-[380px] flex-col gap-3.5 rounded-2xl bg-card p-6"
      >
        <h1 className="text-[26px] leading-8 font-bold">Вход в обмен</h1>
        <p className="text-[15px] leading-5 text-ink-2">
          По номеру телефона сервис узнаёт, чьи это вещи и с кем вы меняетесь. Пароль не нужен.
        </p>

        {/* Поля без подписей и рамок, на серой заливке — как на входе в мобильном вебе Авито:
            их здесь два, и что вводить, видно по плейсхолдеру. */}
        <Input
          tone="soft"
          value={phone}
          onChange={(event) => editPhone(event.target.value)}
          placeholder="Телефон"
          type="tel"
          autoComplete="tel"
          aria-label="Телефон"
          required
        />

        {isNewUser && (
          <>
            <p className="text-[13px] leading-4 font-semibold text-ink-3">
              Такого номера ещё нет — заполните имя, и мы вас заведём
            </p>
            <Input
              tone="soft"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Как вас зовут"
              aria-label="Имя"
              required
            />
          </>
        )}

        {message && (
          <p role="alert" className="text-[13px] leading-4 font-semibold text-stop">
            {message}
          </p>
        )}

        <Button type="submit" fullWidth disabled={enter.isPending || phone.trim() === ''}>
          {isNewUser ? 'Зарегистрироваться' : 'Войти'}
        </Button>

        {/* Демо-номера бэкенда: жюри открывает ссылку и заходит в один клик, а не выясняет,
            какие номера засеяны при старте. Подписаны номером, а не именем: кто за ним стоит,
            знает только база, и после пересева выдуманное имя оказывалось чужим. */}
        <div className="flex flex-col gap-1.5 border-t border-line pt-3.5">
          <span className="text-xs font-semibold text-ink-2">Демо-пользователи</span>
          <div className="flex flex-wrap gap-1.5">
            {DEMO_USERS.map((user) => (
              <button
                key={user.phone}
                type="button"
                onClick={() => {
                  editPhone(user.phone)
                  // Демо-пользователь у бэкенда уже есть — это всегда вход, не регистрация.
                  enter.mutate({ phone: user.phone })
                }}
                className="cursor-pointer rounded-chip border border-line px-2.5 py-1 text-[12.5px] font-bold hover:border-focus"
              >
                {accountLabel(user.phone, user.isAdmin)}
              </button>
            ))}
          </div>
        </div>
      </form>
    </div>
  )
}
