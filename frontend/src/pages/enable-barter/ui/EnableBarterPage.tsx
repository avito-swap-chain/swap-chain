import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { getMyItems, GivenItemCard, itemKeys, setItemWish } from '@/entities/item'
import { DescribeWishForm, WishSaved } from '@/features/describe-wish'
import { Notice, Screen, ScreenHeader } from '@/shared/ui'

/**
 * Включение обмена у уже размещённого объявления — главный вход в сервис.
 * Вещь заводить не надо, нужно только сказать, что хочется взамен.
 */
export function EnableBarterPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [saved, setSaved] = useState(false)
  const { data, isPending, isError } = useQuery({ queryKey: itemKeys.my(), queryFn: getMyItems })
  const item = data?.find((candidate) => candidate.id === id)

  return (
    <Screen>
      {/* После сохранения назад возвращаться некуда: желание уже уехало в подбор. */}
      <ScreenHeader title="Готов обменять" onBack={saved ? undefined : () => navigate('/')} />

      <main className="flex flex-1 flex-col p-4">
        {isPending && <Notice>Загрузка…</Notice>}
        {/* Сбой запроса не равен «объявления нет»: на упавшей сети вещь на месте. */}
        {isError && <Notice tone="error">Не удалось загрузить объявление</Notice>}
        {!isPending && !isError && !item && <Notice tone="error">Объявление не найдено</Notice>}

        {item && saved && <WishSaved title="Обмен включён" />}

        {item && !saved && (
          <div className="flex flex-1 flex-col gap-4">
            <GivenItemCard item={item} />

            <DescribeWishForm
              give={item}
              submitLabel="Включить обмен"
              pendingLabel="Включаем…"
              onSubmit={(wish) => setItemWish(item.id, wish)}
              onDone={() => setSaved(true)}
            />
          </div>
        )}
      </main>
    </Screen>
  )
}
