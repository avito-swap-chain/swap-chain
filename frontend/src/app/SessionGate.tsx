import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { LoginPage } from '@/pages/login'
import { getCurrentUser, sessionKeys } from '@/shared/model/session'

/**
 * Кабинет открывается только тому, кто вошёл. На моках вход не нужен: там «я» — выбранная
 * персона, и `getCurrentUser` никогда не отдаёт `null`, поэтому гейт прозрачен и демо
 * по-прежнему открывается по ссылке без бэкенда.
 */
export function SessionGate({ children }: { children: ReactNode }) {
  const { data: user, isPending } = useQuery({
    queryKey: sessionKeys.current(),
    queryFn: getCurrentUser,
  })

  // Пустой экран на время проверки: мигнуть формой входа тому, кто уже вошёл, хуже,
  // чем показать паузу в один запрос.
  if (isPending) return null

  return user ? children : <LoginPage />
}
