import { useRef, useState, type ChangeEvent, type SubmitEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import {
  CATEGORIES,
  CONDITIONS,
  CONDITION_LABEL,
  descriptionQuality,
  recognizeItem,
  type DescriptionQuality,
  type ItemCondition,
  type ItemDraft,
} from '@/entities/item'
import {
  Button,
  Field,
  IconImage,
  Input,
  Select,
  Status,
  Textarea,
  type StatusTone,
} from '@/shared/ui'
import { recognitionPatch, type RecognizedField } from '../lib/recognitionPatch'

/**
 * Что человек видит вместо порогов длины. Формулировки про подбор, а не про символы:
 * считать буквы — не его работа, ему важно, найдётся ли обмен.
 */
const QUALITY: Record<DescriptionQuality, { tone: StatusTone; label: string }> = {
  short: { tone: 'stop', label: 'Слишком коротко для подбора' },
  fair: { tone: 'attention', label: 'Добавьте деталей — найдётся точнее' },
  good: { tone: 'ok', label: 'Хорошее описание' },
}

/** Первый шаг публикации: сама вещь, без желания. */
export type ItemFormValues = Omit<ItemDraft, 'wish'>

interface AddItemFormProps {
  /** Заполненные ранее значения — чтобы возврат со второго шага не терял ввод. */
  initial?: ItemFormValues
  onSubmit: (values: ItemFormValues) => void
}

export function AddItemForm({ initial, onSubmit }: AddItemFormProps) {
  const [values, setValues] = useState<ItemFormValues>(
    initial ?? { title: '', category: CATEGORIES[0], condition: 'good' },
  )

  // Поля, которые человек заполнил сам: распознавание приходит с задержкой и их не трогает.
  const edited = useRef(new Set<RecognizedField>())

  const patch = (part: Partial<ItemFormValues>) => setValues((prev) => ({ ...prev, ...part }))

  const edit = (field: RecognizedField, part: Partial<ItemFormValues>) => {
    edited.current.add(field)
    patch(part)
  }

  // Распознавание — подсказка: не удалось, значит форма обычная и заполняется руками.
  const recognition = useMutation({
    mutationFn: (file: File) => recognizeItem(file),
    onSuccess: (recognized) => patch(recognitionPatch(recognized, edited.current)),
  })

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

  const quality = values.description?.trim()
    ? QUALITY[descriptionQuality(values.description)]
    : undefined

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
      {recognition.isSuccess && (
        <Status tone="muted">Заполнили по фото — проверьте и поправьте</Status>
      )}
      {recognition.isError && <Status tone="muted">Не узнали вещь на фото — заполните сами</Status>}

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
      <Field label="Описание">
        <Textarea
          value={values.description ?? ''}
          onChange={(event) => patch({ description: event.target.value })}
          placeholder="Что за вещь, как долго у вас, что в комплекте"
          rows={3}
          maxLength={4000}
          disabled={recognition.isPending}
        />
        {/* Подсказка появляется только когда есть что оценивать: у пустого поля она читалась бы
            как упрёк за то, что человек ещё не начал. */}
        {quality && <Status tone={quality.tone}>{quality.label}</Status>}
      </Field>

      <Field label="Категория">
        <Select
          value={values.category}
          onChange={(event) => edit('category', { category: event.target.value })}
          disabled={recognition.isPending}
        >
          {CATEGORIES.map((category) => (
            <option key={category} value={category}>
              {category}
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
