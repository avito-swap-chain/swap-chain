import { useNavigate, useRouteError } from 'react-router-dom'
import { Button } from '@/shared/ui'

/**
 * Экран не открылся. Роутер перехватывает исключения сам и без `errorElement` показал бы
 * свою служебную страницу с трассировкой — человеку она ничего не объясняет, а на демо
 * выглядит как падение сервиса.
 *
 * Отсюда два выхода: вернуться в кабинет (роутер сбрасывает ошибку при переходе) или
 * перезагрузить, если сломалось что-то общее.
 */
export function RouteError() {
  const error = useRouteError()
  const navigate = useNavigate()

  // Причину оставляем в консоли: сборщика ошибок в MVP нет, а без неё нечего отлаживать.
  console.error('Экран не открылся:', error)

  return (
    <div className="flex min-h-svh items-center justify-center bg-page px-4">
      <div className="flex w-full max-w-[380px] flex-col gap-3.5 rounded-2xl bg-card p-6 text-center">
        <h1 className="text-[20px] leading-6 font-bold">Не удалось открыть раздел</h1>
        <p className="text-[14px] leading-5 text-ink-2">
          Остальные разделы работают — вернитесь в кабинет или обновите страницу.
        </p>
        <Button fullWidth onClick={() => navigate('/')}>
          Вернуться в кабинет
        </Button>
        <Button variant="secondary" fullWidth onClick={() => window.location.reload()}>
          Обновить страницу
        </Button>
      </div>
    </div>
  )
}
