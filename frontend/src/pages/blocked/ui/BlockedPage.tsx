import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { chainKeys } from '@/entities/chain'
import { getBlocked, unblockUser, userKeys } from '@/entities/user'
import { Avatar, Button, Notice, Screen } from '@/shared/ui'

/**
 * Чёрный список — обратная сторона блокировки: заблокировать человека можно было из его
 * профиля, а увидеть, кого уже заблокировал, было негде. Для сервиса, который сводит
 * незнакомых людей, это половина механизма: список сам решает, кто не попадёт в подбор.
 *
 * Разблокировка возвращает человека в подбор, но не воскрешает распущенные цепочки —
 * так работает бэкенд, и об этом сказано прямо здесь, а не после нажатия.
 */
export function BlockedPage() {
  const queryClient = useQueryClient()
  const { data, isPending, isError } = useQuery({
    queryKey: userKeys.blocked(),
    queryFn: getBlocked,
  })

  const unblock = useMutation({
    mutationFn: (userId: string) => unblockUser(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: userKeys.blocked() })
      // Разблокировка снова пускает человека в подбор — список обменов после неё устарел.
      queryClient.invalidateQueries({ queryKey: chainKeys.all })
    },
  })

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">
          Чёрный список
        </h1>

        {isPending && <Notice>Загрузка…</Notice>}
        {isError && <Notice tone="error">Не удалось загрузить чёрный список</Notice>}

        {data && data.length === 0 && (
          <p className="py-4 text-[13px] text-ink-2">
            Здесь пусто. Заблокировать человека можно в его профиле — после этого вы не встретитесь
            ни в подборе, ни в одном обмене.
          </p>
        )}

        {data && data.length > 0 && (
          <>
            <p className="text-[13px] leading-4 text-ink-2">
              С этими людьми сервис не соберёт общий обмен. Разблокировка вернёт их в подбор, но
              отменённые цепочки не восстановит.
            </p>

            <ul className="flex flex-col">
              {data.map((user) => (
                <li
                  key={user.id}
                  className="flex items-center gap-3 border-b border-line py-3 last:border-0"
                >
                  <Avatar name={user.name} className="size-10 text-[15px]" />
                  <Link
                    to={`/users/${user.id}`}
                    className="flex-1 text-[15px] leading-5 font-bold hover:text-brand"
                  >
                    {user.name}
                  </Link>
                  <Button
                    variant="secondary"
                    disabled={unblock.isPending}
                    onClick={() => unblock.mutate(user.id)}
                  >
                    Разблокировать
                  </Button>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </Screen>
  )
}
