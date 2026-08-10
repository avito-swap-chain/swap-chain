import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { chainKeys } from '@/entities/chain'
import { itemKeys } from '@/entities/item'
import { subscribeToBackendEvents } from '@/shared/api/events'
import { isBackendConnected } from '@/shared/config/backend'

/**
 * Слушает поток событий бэкенда и помечает затронутые данные устаревшими — перезапрашивает
 * их TanStack Query сам, и только для тех экранов, что сейчас открыты.
 *
 * Живёт в `app`, а не в экранах: подписка должна быть одна на приложение. Держи её страница —
 * поток рвался бы на каждом переходе, а события, пришедшие между переходами, терялись бы.
 *
 * На моках подписываться не к кому, поэтому там ничего не открывается.
 */
export function BackendEvents() {
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!isBackendConnected) return

    const invalidateItems = () => queryClient.invalidateQueries({ queryKey: itemKeys.my() })
    const invalidateChains = () => queryClient.invalidateQueries({ queryKey: chainKeys.all })

    return subscribeToBackendEvents({
      onItems: invalidateItems,
      onChains: invalidateChains,
      // Пропущенное за время обрыва сервер не переотправит — после подключения
      // перечитываем всё, иначе экран останется с состоянием до разрыва.
      onConnected: () => {
        invalidateItems()
        invalidateChains()
      },
    })
  }, [queryClient])

  return null
}
