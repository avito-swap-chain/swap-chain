import { useEffect } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { chainKeys } from '@/entities/chain'
import { itemKeys } from '@/entities/item'
import { messageKeys } from '@/entities/message'
import { notificationKeys } from '@/entities/notification'
import { subscribeToBackendEvents } from '@/shared/api/events'
import { isBackendConnected } from '@/shared/config/backend'
import { getCurrentUser, sessionKeys } from '@/shared/model/session'

/**
 * Слушает поток событий бэкенда и помечает затронутые данные устаревшими — перезапрашивает
 * их TanStack Query сам, и только для тех экранов, что сейчас открыты.
 *
 * Живёт в `app`, а не в экранах: подписка должна быть одна на приложение. Держи её страница —
 * поток рвался бы на каждом переходе, а события, пришедшие между переходами, терялись бы.
 *
 * Открывается только когда есть сессия, и переоткрывается при её смене. Иначе поток
 * запрашивался бы до входа, получал 401 — и на этом всё: `EventSource` переподключается сам
 * после обрыва соединения, но не после ошибочного ответа, он просто закрывается навсегда.
 * До перезагрузки страницы события не приходили бы вовсе.
 *
 * На моках подписываться не к кому, поэтому там ничего не открывается.
 */
export function BackendEvents() {
  const queryClient = useQueryClient()
  const { data: user } = useQuery({
    queryKey: sessionKeys.current(),
    queryFn: getCurrentUser,
    enabled: isBackendConnected,
  })

  const userId = user?.id

  useEffect(() => {
    if (!isBackendConnected || !userId) return

    const invalidateItems = () => queryClient.invalidateQueries({ queryKey: itemKeys.my() })
    const invalidateChains = () => queryClient.invalidateQueries({ queryKey: chainKeys.all })
    const invalidateNotifications = () =>
      queryClient.invalidateQueries({ queryKey: notificationKeys.all })
    // Поддержка ответила: перечитываем и список переписок (там её строка и счётчик),
    // и открытую ленту — она висит на long-poll, но событие приходит раньше ответа.
    const invalidateMessages = () => queryClient.invalidateQueries({ queryKey: messageKeys.all })

    return subscribeToBackendEvents({
      onItems: invalidateItems,
      // Цепочка тянет за собой вещи: завершение обмена переводит их в `EXCHANGED` той же
      // транзакцией, и без перечитывания списка вещь осталась бы «в цепочке» навсегда.
      onChains: () => {
        invalidateChains()
        invalidateItems()
      },
      onNotifications: invalidateNotifications,
      onSupport: invalidateMessages,
      // Пропущенное за время обрыва сервер не переотправит — после подключения
      // перечитываем всё, иначе экран останется с состоянием до разрыва.
      onConnected: () => {
        invalidateItems()
        invalidateChains()
        invalidateNotifications()
        invalidateMessages()
      },
    })
  }, [queryClient, userId])

  return null
}
