import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { DescriptionField, editItem, getMyItems, itemKeys, type Item } from '@/entities/item'
import { DescribeWishForm } from '@/features/describe-wish'
import { Notice, Screen, ScreenHeader } from '@/shared/ui'

/**
 * Правка размещённого объявления: описание и желание. Отдельный экран нужен потому, что
 * иначе поправить формулировку можно было только снятием с обмена и повторным включением —
 * а снятие отменяет предложения с этой вещью у других людей.
 *
 * Название и фото здесь не меняются: `UpdateItemRequest` (0.6.0) таких полей не принимает.
 */
export function EditItemPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { data, isPending, isError } = useQuery({ queryKey: itemKeys.my(), queryFn: getMyItems })
  const item = data?.find((candidate) => candidate.id === id)

  return (
    <Screen>
      <ScreenHeader title="Редактирование" onBack={() => navigate('/')} />

      <main className="flex flex-1 flex-col p-4">
        {isPending && <Notice>Загрузка…</Notice>}
        {/* «Не загрузилось» и «нет такого» — разные вещи: на сбое сети объявление никуда
            не делось, и говорить человеку, что оно пропало, нельзя. */}
        {isError && <Notice tone="error">Не удалось загрузить объявление</Notice>}
        {!isPending && !isError && !item && <Notice tone="error">Объявление не найдено</Notice>}

        {item && <EditItemFields item={item} onDone={() => navigate('/')} />}
      </main>
    </Screen>
  )
}

/**
 * Поля отдельным компонентом: он монтируется, когда вещь уже загружена, — иначе начальные
 * значения формы застыли бы пустыми, какими были на первом рендере.
 */
function EditItemFields({ item, onDone }: { item: Item; onDone: () => void }) {
  const [description, setDescription] = useState(item.description ?? '')

  return (
    <div className="flex flex-1 flex-col gap-3.5">
      <DescriptionField value={description} onChange={setDescription} />

      <DescribeWishForm
        give={item}
        initial={item.wish}
        submitLabel="Сохранить"
        pendingLabel="Сохраняем…"
        onSubmit={(wish) => editItem(item.id, { wish, description })}
        onDone={onDone}
      />
    </div>
  )
}
