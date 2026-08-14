import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { LoginPage } from '@/pages/login'
import { describeError } from '@/shared/api/describeError'
import { getCurrentUser, sessionKeys } from '@/shared/model/session'
import { Button } from '@/shared/ui'

/**
 * Кабинет открывается только тому, кто вошёл. На моках вход не нужен: там «я» — выбранная
 * персона, и `getCurrentUser` никогда не отдаёт `null`, поэтому гейт прозрачен и демо
 * по-прежнему открывается по ссылке без бэкенда.
 */
export function SessionGate({ children }: { children: ReactNode }) {
  const {
    data: user,
    isPending,
    isError,
    error,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: sessionKeys.current(),
    queryFn: getCurrentUser,
    // Один повтор вместо трёх: до сервера либо достучались, либо нет, и минуту ждать
    // молчаливый экран человеку незачем — лучше сказать, что случилось.
    retry: 1,
  })

  // Пустой экран на время проверки: мигнуть формой входа тому, кто уже вошёл, хуже,
  // чем показать паузу в один запрос.
  if (isPending) return null

  /**
   * «Не вошёл» и «сервис не отвечает» — разные вещи, а выглядели одинаково: при упавшем
   * бэкенде человек получал форму входа, вводил номер и молча получал отказ. Теперь
   * говорим прямо, что дело не в нём, и даём повторить, не перезагружая страницу.
   */
  if (isError) {
    return (
      <div className="mx-auto flex min-h-svh max-w-md flex-col justify-center gap-4 p-6 text-center">
        <h1 className="text-[22px] leading-7 font-bold">Сервис сейчас недоступен</h1>
        <p className="text-[15px] leading-5 text-ink-2">
          {describeError(error)}. Ваши вещи и обмены на месте — до них не дотянуться прямо сейчас.
        </p>
        <div>
          <Button disabled={isFetching} onClick={() => refetch()}>
            {isFetching ? 'Пробуем…' : 'Попробовать снова'}
          </Button>
        </div>
      </div>
    )
  }

  return user ? children : <LoginPage />
}
