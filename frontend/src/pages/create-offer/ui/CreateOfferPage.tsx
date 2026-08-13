import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createItem, GivenItemCard } from '@/entities/item'
import { AddItemFlow, type ItemFormValues } from '@/features/add-item'
import { DescribeWishForm, WishSaved } from '@/features/describe-wish'
import { Screen, ScreenHeader, Status } from '@/shared/ui'

/**
 * Публикация вещи в два шага: сама вещь → что хочется взамен.
 * Шаги живут на одном роуте — ввод первого шага не теряется при возврате.
 */
export function CreateOfferPage() {
  const navigate = useNavigate()
  const [values, setValues] = useState<ItemFormValues>()
  const [step, setStep] = useState<1 | 2>(1)
  const [saved, setSaved] = useState(false)

  const goBack = () => {
    if (step === 2) setStep(1)
    else navigate('/')
  }

  return (
    <Screen>
      {/* Заголовок один на оба шага: на втором он повторял бы подпись поля «Что хотите взамен»
          прямо под собой и расходился с крошками. Где мы — говорит счётчик шагов. */}
      <ScreenHeader title="Новое объявление" onBack={saved ? undefined : goBack}>
        {!saved && <Status tone="muted">Шаг {step} из 2</Status>}
      </ScreenHeader>

      <main className="flex flex-1 flex-col p-4">
        {saved && <WishSaved title="Объявление опубликовано" />}

        {!saved && step === 2 && values && (
          <div className="flex flex-1 flex-col gap-4">
            <GivenItemCard item={values} />

            <DescribeWishForm
              give={values}
              submitLabel="Опубликовать и искать обмен"
              pendingLabel="Публикуем…"
              onSubmit={(wish) => createItem({ ...values, wish })}
              onDone={() => setSaved(true)}
            />
          </div>
        )}

        {!saved && step !== 2 && (
          <AddItemFlow
            initial={values}
            onSubmit={(next) => {
              setValues(next)
              setStep(2)
            }}
          />
        )}
      </main>
    </Screen>
  )
}
