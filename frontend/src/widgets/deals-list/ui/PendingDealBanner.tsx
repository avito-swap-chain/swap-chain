import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { chainKeys, getMyChains, needsMyAction } from '@/entities/chain'
import { Banner, IconClock } from '@/shared/ui'

/**
 * Зовёт в запущенный обмен, где ход за пользователем: он ещё не отметил, что получил вещь.
 * Предложения сюда не попадают — у них свой блок с ответом прямо в списке, и баннер над ним
 * повторял бы то же самое.
 */
export function PendingDealBanner() {
  const { data } = useQuery({ queryKey: chainKeys.my(), queryFn: getMyChains })

  const pending = data?.find((chain) => chain.status === 'active' && needsMyAction(chain))
  if (!pending) return null

  return (
    <Link
      to={`/exchange/${pending.id}`}
      className="rounded-2xl outline-offset-2 focus-visible:outline-2 focus-visible:outline-brand"
    >
      <Banner tone="info" icon={<IconClock size={20} />}>
        <b className="font-bold">Обмен идёт</b> — отметьте, что получили вещь
      </Banner>
    </Link>
  )
}
