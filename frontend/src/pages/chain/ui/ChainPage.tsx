import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { chainKeys, DealStatusLabel, getChain } from '@/entities/chain'
import { Notice, Screen, ScreenHeader } from '@/shared/ui'
import { ChainBoard } from '@/widgets/chain-board'
import { isBackendConnected } from '@/shared/config/backend'

/**
 * Один роут на всю сделку: предложение, ожидание, передача, завершение и распад —
 * это разные состояния одной цепочки, а не разные экраны.
 * Экран одного решения, поэтому колонка узкая и на широком окне не растягивается.
 */
export function ChainPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()

  const {
    data: chain,
    isPending,
    isError,
  } = useQuery({
    queryKey: chainKeys.detail(id),
    queryFn: () => getChain(id),
    // С бэкендом обновления приносит поток событий (`app/BackendEvents`) — опрашивать
    // сверх него незачем. Опрос остаётся только для моков: там событий не от кого получать,
    // а ответы соседей приходят по таймеру, и без опроса демо стоит на месте.
    refetchInterval: (query) => {
      if (isBackendConnected) return false
      const status = query.state.data?.status
      return status === 'formed' || status === 'active' ? 2000 : false
    },
  })

  return (
    <Screen>
      <ScreenHeader title="Цепочка обмена" onBack={() => navigate('/exchange')}>
        {chain && <DealStatusLabel chain={chain} />}
      </ScreenHeader>

      <main className="flex flex-1 flex-col p-3.5 sm:p-4">
        {isPending && <Notice>Загрузка…</Notice>}
        {isError && <Notice tone="error">Не удалось загрузить цепочку</Notice>}
        {chain && <ChainBoard chain={chain} />}
      </main>
    </Screen>
  )
}
