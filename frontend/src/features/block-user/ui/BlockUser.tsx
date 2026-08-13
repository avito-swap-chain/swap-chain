import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { blockUser, getBlocked, unblockUser, userKeys } from '@/entities/user'
import { genitive } from '@/shared/lib'
import { ActionError, Button, IconBan, TileGroup, TileRow } from '@/shared/ui'

/**
 * Заблокировать соседа по кругу. Обмен сводит незнакомых людей, и отказаться от общения
 * с конкретным человеком — минимальная защита, без которой сервис заставляет терпеть.
 *
 * Действие обратимое, и цена названа до нажатия: подтверждение говорит, что именно
 * произойдёт с текущим обменом, а не просто спрашивает «вы уверены?».
 */
export function BlockUser({ userId, name }: { userId: string; name: string }) {
  const [asking, setAsking] = useState(false)
  const queryClient = useQueryClient()

  const { data: blocked = [] } = useQuery({ queryKey: userKeys.blocked(), queryFn: getBlocked })
  const isBlocked = blocked.includes(userId)

  const { mutate, isPending, error } = useMutation({
    mutationFn: () => (isBlocked ? unblockUser(userId) : blockUser(userId)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: userKeys.blocked() })
      setAsking(false)
    },
  })

  if (isBlocked) {
    return (
      <TileGroup>
        <TileRow icon={<IconBan size={18} />} onClick={() => mutate()}>
          {isPending ? 'Разблокируем…' : `Разблокировать ${genitive(name)}`}
        </TileRow>
      </TileGroup>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <TileGroup>
        <TileRow icon={<IconBan size={18} />} onClick={() => setAsking((open) => !open)}>
          Заблокировать
        </TileRow>
      </TileGroup>

      {asking && (
        <div className="flex flex-col gap-2.5 rounded-card border border-line p-3">
          <p className="text-[13.5px] leading-5 text-ink-2">
            Вы больше не увидите друг друга в подборе и не окажетесь в одном обмене.{' '}
            <b className="font-bold text-ink">Текущий обмен это не отменяет</b> — если вещи уже
            едут, доведите его до конца или выйдите из цепочки отдельно.
          </p>
          <p className="text-[13.5px] leading-5 text-ink-2">Разблокировать можно здесь же.</p>

          <div className="flex gap-2">
            <Button variant="danger" disabled={isPending} onClick={() => mutate()}>
              {isPending ? 'Блокируем…' : 'Заблокировать'}
            </Button>
            <Button variant="ghost" onClick={() => setAsking(false)}>
              Отмена
            </Button>
          </div>

          <ActionError error={error} />
        </div>
      )}
    </div>
  )
}
