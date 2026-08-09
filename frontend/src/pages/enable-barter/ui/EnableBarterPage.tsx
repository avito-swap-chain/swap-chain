import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { getMyItems, itemKeys, setItemWish } from '@/entities/item'
import { DescribeWishForm } from '@/features/describe-wish'
import { Notice, Screen, ScreenHeader } from '@/shared/ui'

/**
 * Включение обмена у уже размещённого объявления — главный вход в сервис.
 * Вещь заводить не надо, нужно только сказать, что хочется взамен.
 */
export function EnableBarterPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { data, isPending } = useQuery({ queryKey: itemKeys.my(), queryFn: getMyItems })
  const item = data?.find((candidate) => candidate.id === id)

  return (
    <Screen>
      <ScreenHeader title="Готов обменять" onBack={() => navigate('/')} />

      <main className="flex flex-1 flex-col p-4">
        {isPending && <Notice>Загрузка…</Notice>}
        {!isPending && !item && <Notice tone="error">Объявление не найдено</Notice>}

        {item && (
          <DescribeWishForm
            give={item}
            submitLabel="Включить обмен"
            pendingLabel="Включаем…"
            onSubmit={(wish) => setItemWish(item.id, wish)}
            onDone={() => navigate('/')}
          />
        )}
      </main>
    </Screen>
  )
}
