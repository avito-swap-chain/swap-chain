import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import {
  categoryKeys,
  DescriptionField,
  editItem,
  getCategories,
  getMyItems,
  itemKeys,
  type Item,
} from '@/entities/item'
import { DescribeWishForm } from '@/features/describe-wish'
import { WithdrawItem } from '@/features/withdraw-item'
import { Field, Input, Notice, Screen, ScreenHeader, Select } from '@/shared/ui'

/**
 * Правка размещённого объявления: название, описание и желание. Отдельный экран нужен потому,
 * что иначе поправить формулировку можно было только снятием с обмена и повторным включением —
 * а снятие отменяет предложения с этой вещью у других людей.
 *
 * Фото здесь пока не меняется: `imageUrls` в `UpdateItemRequest` не принимается.
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
  const [title, setTitle] = useState(item.title)
  const [description, setDescription] = useState(item.description ?? '')
  const [categoryId, setCategoryId] = useState(item.categoryId)

  const { data: categories = [] } = useQuery({
    queryKey: categoryKeys.list(),
    queryFn: getCategories,
  })

  return (
    <div className="flex flex-1 flex-col gap-3.5">
      {/* Название правится с тех пор, как бэкенд принял `offerTitle`: до этого объявление
          можно было переписать наполовину, и именно на это указал ментор. */}
      <Field label="Название">
        <Input
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="Например, горный велосипед"
          required
        />
      </Field>

      <DescriptionField value={description} onChange={setDescription} />

      {/* Смена категории перезапускает разбор вещи на бэкенде — она участвует в подборе. */}
      <Field label="Категория">
        <Select
          value={categoryId ?? ''}
          onChange={(event) => setCategoryId(Number(event.target.value) || undefined)}
          disabled={categories.length === 0}
        >
          <option value="" disabled>
            Выберите категорию
          </option>
          {categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </Select>
      </Field>

      <DescribeWishForm
        give={{ ...item, title }}
        initial={item.wish}
        submitLabel="Сохранить"
        pendingLabel="Сохраняем…"
        onSubmit={(wish) => editItem(item.id, { wish, description, title, categoryId })}
        onDone={onDone}
      />

      {/* Снятие с обмена переехало сюда с карточки: на списке оно стояло рядом с правкой
          и задевалось мимоходом, хотя отменяет предложения с этой вещью у других людей.
          Здесь оно на своём месте — под формой, отдельно от сохранения, и с ценой рядом. */}
      <div className="mt-2 flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-t border-line pt-3.5">
        <span className="text-[13px] leading-4 text-ink-2">
          Больше не хотите меняться? Вещь останется объявлением.
        </span>
        <WithdrawItem itemId={item.id} onDone={onDone} />
      </div>
    </div>
  )
}
