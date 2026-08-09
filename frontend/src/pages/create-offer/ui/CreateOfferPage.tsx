import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createItem } from '@/entities/item'
import { AddItemForm, type ItemFormValues } from '@/features/add-item'
import { DescribeWishForm } from '@/features/describe-wish'
import { Screen, ScreenHeader, Status } from '@/shared/ui'

/**
 * Публикация вещи в два шага: сама вещь → что хочется взамен.
 * Шаги живут на одном роуте — ввод первого шага не теряется при возврате.
 */
export function CreateOfferPage() {
  const navigate = useNavigate()
  const [values, setValues] = useState<ItemFormValues>()
  const [step, setStep] = useState<1 | 2>(1)

  const goBack = () => {
    if (step === 2) setStep(1)
    else navigate('/')
  }

  return (
    <Screen>
      {/* Заголовок один на оба шага: на втором он повторял бы подпись поля «Что хотите взамен»
          прямо под собой и расходился с крошками. Где мы — говорит счётчик шагов. */}
      <ScreenHeader title="Новое объявление" onBack={goBack}>
        <Status tone="muted">Шаг {step} из 2</Status>
      </ScreenHeader>

      <main className="flex flex-1 flex-col p-4">
        {step === 2 && values ? (
          <DescribeWishForm
            give={values}
            submitLabel="Опубликовать и искать обмен"
            pendingLabel="Публикуем…"
            onSubmit={(wish) => createItem({ ...values, wish })}
            onDone={() => navigate('/')}
          />
        ) : (
          <AddItemForm
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
