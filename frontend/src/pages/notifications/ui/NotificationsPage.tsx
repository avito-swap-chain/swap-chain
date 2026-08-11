import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import {
  getNotifications,
  NotificationCard,
  notificationKeys,
  readNotifications,
} from '@/entities/notification'
import { Card, EmptyState, Notice, Screen } from '@/shared/ui'

/**
 * Раздел «Уведомления» — то, что произошло, пока человека не было: ему написали, цепочка
 * сменила состояние, вариант перестал быть возможным. Открытие списка гасит счётчик:
 * человек уже увидел всё, что здесь есть.
 */
export function NotificationsPage() {
  const { data, isPending, isError } = useQuery({
    queryKey: notificationKeys.list(),
    queryFn: getNotifications,
  })

  const queryClient = useQueryClient()
  const unreadCount = data?.totalUnread ?? 0

  useEffect(() => {
    if (unreadCount === 0) return
    readNotifications().then(() => {
      queryClient.invalidateQueries({ queryKey: notificationKeys.list() })
    })
  }, [unreadCount, queryClient])

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3.5 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">
          Уведомления
        </h1>

        {isPending && <Notice>Загрузка…</Notice>}
        {isError && <Notice tone="error">Не удалось загрузить уведомления</Notice>}

        {/* В карточке, а не текстом по серому: без неё пустой раздел выглядел брошенной
            страницей — заголовок пустого состояния почти совпадал по весу с заголовком
            раздела и читался как его повтор, а вокруг не было ничего. */}
        {data && data.items.length === 0 && (
          <Card className="px-4 lg:px-6">
            <EmptyState
              title="Уведомлений пока нет"
              description="Здесь появится то, что произошло без вас: ответы участников, собравшиеся цепочки и отменённые варианты."
            />
          </Card>
        )}

        {data && data.items.length > 0 && (
          <div className="flex flex-col">
            {data.items.map((notification) => (
              <NotificationCard key={notification.id} notification={notification} />
            ))}
          </div>
        )}
      </div>
    </Screen>
  )
}
