import { useRef, useState, type ChangeEvent, type SubmitEvent } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import {
  categoryKeys,
  CONDITIONS,
  CONDITION_LABEL,
  DescriptionField,
  getCategories,
  matchCategory,
  recognizeItem,
  type ItemCondition,
  type ItemDraft,
} from '@/entities/item'
import { isBackendConnected } from '@/shared/config/backend'
import { ActionError, Button, Field, IconImage, Input, Select, Status } from '@/shared/ui'
import { recognitionPatch, type RecognizedField } from '../lib/recognitionPatch'

/** Первый шаг публикации: сама вещь, без желания. */
export type ItemFormValues = Omit<ItemDraft, 'wish'>

interface AddItemFormProps {
  /** Заполненные ранее значения — чтобы возврат со второго шага не терял ввод. */
  initial?: ItemFormValues
  onSubmit: (values: ItemFormValues) => void
}

export function AddItemForm({ initial, onSubmit }: AddItemFormProps) {
  // Справочник категорий — с бэкенда: по нему же идёт подбор, и свой список тут завести
  // значит выбирать из одного, а искать по другому.
  const { data: categories = [] } = useQuery({
    queryKey: categoryKeys.list(),
    queryFn: getCategories,
  })

  const [values, setValues] = useState<ItemFormValues>(
    initial ?? { title: '', category: '', condition: 'good' },
  )

  // Поля, которые человек заполнил сам: распознавание приходит с задержкой и их не трогает.
  const edited = useRef(new Set<RecognizedField>())

  const patch = (part: Partial<ItemFormValues>) => setValues((prev) => ({ ...prev, ...part }))

  const edit = (field: RecognizedField, part: Partial<ItemFormValues>) => {
    edited.current.add(field)
    patch(part)
  }

  /**
   * Распознавание — подсказка: не удалось, значит форма обычная и заполняется руками.
   *
   * На моках ответ мгновенный, и поля заполняются сами — человек ещё не начал печатать.
   * С бэкендом модель думает несколько секунд, за которые он уже что-то ввёл: там результат
   * показывается предложением рядом с полями, а не переписывает набранное.
   */
  const recognition = useMutation({
    mutationFn: (file: File) => recognizeItem(file),
    onSuccess: (recognized) => {
      // Фото уже в хранилище бэкенда — при публикации его не надо грузить второй раз.
      if (recognized.imageUrl) patch({ uploadedUrl: recognized.imageUrl })
      if (!isBackendConnected) patch(recognitionPatch(recognized, edited.current))
    },
  })

  /** Принять подсказку модели: то, что человек уже написал сам, остаётся как есть. */
  const applyRecognition = () => {
    if (!recognition.data) return

    const suggested = matchCategory(recognition.data.category, categories)

    patch({
      ...recognitionPatch(recognition.data, edited.current),
      // Модель отвечает названием из своего промпта, а не идентификатором справочника:
      // не нашлось пары — категорию не трогаем, описание и состояние всё равно полезны.
      ...(suggested && !edited.current.has('category')
        ? { category: suggested.name, categoryId: suggested.id }
        : {}),
    })
    recognition.reset()
  }

  const pickPhoto = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    // Ссылку на прежнее фото освобождаем, иначе замена фото копит их в памяти.
    if (values.photoUrl) URL.revokeObjectURL(values.photoUrl)
    // Сам файл несём дальше: предпросмотр рисуется по `blob:`-ссылке, а бэкенду нужен файл.
    patch({ photoUrl: URL.createObjectURL(file), photoFile: file })
    recognition.mutate(file)
  }

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    onSubmit({ ...values, title: values.title.trim(), description: values.description?.trim() })
  }

  return (
    <form onSubmit={submit} className="flex flex-1 flex-col gap-3.5">
      <label className="flex cursor-pointer flex-col items-center gap-2 rounded-2xl border-[1.5px] border-dashed border-line p-5 text-center text-ink-3">
        {values.photoUrl ? (
          <img src={values.photoUrl} alt="" className="size-28 rounded-xl object-cover" />
        ) : (
          <IconImage size={28} />
        )}
        <b className="text-sm font-bold text-ink">
          {values.photoUrl ? 'Заменить фото' : 'Добавьте фото вещи'}
        </b>
        <span className="text-[12.5px]">С фото обмен находится быстрее</span>
        <input type="file" accept="image/*" className="sr-only" onChange={pickPhoto} />
      </label>

      {recognition.isPending && <Status>Распознаём вещь на фото…</Status>}

      {/* На моках поля заполнены сразу — остаётся сказать об этом. */}
      {recognition.isSuccess && !isBackendConnected && (
        <Status tone="muted">Заполнили по фото — проверьте и поправьте</Status>
      )}

      {/* С бэкендом ответ приходит через несколько секунд: показываем предложением,
          а не переписываем то, что человек успел ввести. Решает он. */}
      {recognition.isSuccess && isBackendConnected && recognition.data && (
        <div className="flex flex-col gap-2 rounded-card border border-line bg-card p-3">
          <span className="font-sans text-xs font-semibold text-ink-2">Распознали по фото</span>

          <p className="text-[13.5px] leading-5 text-ink">
            {recognition.data.description ?? 'Описание подобрать не удалось'}
          </p>

          <p className="text-[12.5px] text-ink-3">
            {[
              recognition.data.category,
              recognition.data.condition && CONDITION_LABEL[recognition.data.condition],
            ]
              .filter(Boolean)
              .join(' · ')}
          </p>

          <Button
            variant="ghost"
            onClick={applyRecognition}
            className="self-start px-3 py-2 text-[13.5px]"
          >
            Заполнить этим
          </Button>

          <span className="text-[12px] text-ink-3">
            Название модель не придумывает — впишите сами
          </span>
        </div>
      )}

      {recognition.isError && (
        <ActionError error={recognition.error} conflict="Это фото распознать не получилось" />
      )}

      <Field label="Название">
        <Input
          value={values.title}
          onChange={(event) => edit('title', { title: event.target.value })}
          placeholder="Например, горный велосипед"
          disabled={recognition.isPending}
          required
        />
      </Field>

      {/* Описание не обязательно, но по нему подбор и ищет: бэкенд векторизует
          название вместе с описанием, и без него у вещи остаётся одно название. */}
      <DescriptionField
        value={values.description ?? ''}
        onChange={(description) => patch({ description })}
        disabled={recognition.isPending}
      />

      <Field label="Категория">
        <Select
          value={values.categoryId ?? ''}
          onChange={(event) => {
            const chosen = categories.find((item) => String(item.id) === event.target.value)
            edit('category', { category: chosen?.name ?? '', categoryId: chosen?.id })
          }}
          disabled={recognition.isPending || categories.length === 0}
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

      <Field label="Состояние">
        <Select
          value={values.condition}
          onChange={(event) =>
            edit('condition', { condition: event.target.value as ItemCondition })
          }
          disabled={recognition.isPending}
        >
          {CONDITIONS.map((condition) => (
            <option key={condition} value={condition}>
              {CONDITION_LABEL[condition]}
            </option>
          ))}
        </Select>
      </Field>

      <div className="mt-auto pt-2">
        <Button type="submit" fullWidth disabled={recognition.isPending || !values.title.trim()}>
          Далее: что хотите взамен
        </Button>
      </div>
    </form>
  )
}
