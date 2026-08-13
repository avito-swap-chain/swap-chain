import { useRef, useState, type SubmitEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { itemKeys, suggestWish, type GivenItem } from '@/entities/item'
import { Button, IconClose, IconPlus, Input } from '@/shared/ui'

/** Строка формы: вариант желания + стабильный ключ, чтобы удаление не путало поля React. */
interface WishRow {
  key: number
  description: string
}

/** Больше пяти вариантов человек уже не держит в голове, а форма превращается в простыню. */
const MAX_VARIANTS = 5

const emptyRow = (key: number): WishRow => ({ key, description: '' })

interface DescribeWishFormProps {
  /** Что человек отдаёт: по названию и категории собираются подсказки. */
  give: GivenItem
  /**
   * Уже сохранённое желание. При правке форма открывается с ним: иначе «поменять один
   * вариант» означало бы набрать заново все, и человек терял бы то, что уже указал.
   */
  initial?: string[]
  submitLabel: string
  pendingLabel: string
  /**
   * Как сохранить желание. Форма одна на оба входа в сервис — публикацию новой вещи
   * и включение обмена у уже размещённого объявления; отличается только сохранение.
   */
  onSubmit: (wish: string[]) => Promise<unknown>
  onDone: () => void
}

/**
 * Желание: что пользователь хочет получить взамен. Вариантов может быть несколько, и это
 * не украшение — каждый вариант отдельное ребро графа и отдельный шанс замкнуть цикл.
 * Широту даём количеством конкретных вариантов, а не размытой формулировкой в одном поле.
 *
 * Категорию варианта форма не спрашивает: в контракт уезжает только формулировка, а к какому
 * разделу она относится, бэкенд определяет сам — на этом и держится подбор. Выбор из списка
 * был бы вопросом, ответ на который никуда не идёт.
 */
export function DescribeWishForm({
  give,
  initial,
  submitLabel,
  pendingLabel,
  onSubmit,
  onDone,
}: DescribeWishFormProps) {
  const [rows, setRows] = useState<WishRow[]>(
    initial?.length
      ? initial.map((description, index) => ({ description, key: index }))
      : [emptyRow(0)],
  )
  // Ключи новых строк продолжают занятые: совпадение спутало бы React поля местами.
  const nextKey = useRef(rows.length)
  const queryClient = useQueryClient()

  // Пустые строки не сохраняем: добавить вариант и передумать — нормальный ход.
  const filled = rows
    .map((row) => row.description.trim())
    .filter((description) => description !== '')

  // Подсказки желания — из поисков и избранного человека, см. `suggestWish`. Уже выбранные
  // варианты уходят из подсказок, поэтому они и лежат в ключе запроса.
  const { data: suggestions = [] } = useQuery({
    queryKey: itemKeys.suggestions(give.title, filled),
    queryFn: () => suggestWish(give, filled),
    placeholderData: (previous) => previous,
  })

  const { mutate, isPending, isError } = useMutation({
    mutationFn: () => onSubmit(filled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: itemKeys.my() })
      onDone()
    },
  })

  const setDescription = (key: number, description: string) =>
    setRows((prev) => prev.map((row) => (row.key === key ? { ...row, description } : row)))

  // Есть куда положить ещё один вариант: свободная строка или место под новую.
  const hasRoom = rows.length < MAX_VARIANTS || rows.some((row) => row.description.trim() === '')

  /** Подсказка занимает первую пустую строку, а если пустых нет — становится новым вариантом. */
  const addSuggestion = (description: string) => {
    const key = nextKey.current++

    setRows((prev) => {
      const empty = prev.find((row) => row.description.trim() === '')
      if (empty) return prev.map((row) => (row.key === empty.key ? { ...row, description } : row))

      return prev.length < MAX_VARIANTS ? [...prev, { description, key }] : prev
    })
  }

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    mutate()
  }

  return (
    <form onSubmit={submit} className="flex flex-1 flex-col gap-4">
      <h2 className="text-[19px] leading-6 font-bold">Что хотите взамен?</h2>

      <div className="flex flex-col gap-2">
        {rows.map((row, index) => (
          <div key={row.key} className="flex items-center gap-2">
            <Input
              value={row.description}
              onChange={(event) => setDescription(row.key, event.target.value)}
              aria-label={index === 0 ? 'Что хотите взамен' : `Вариант ${index + 1}`}
              placeholder={index === 0 ? 'Например, игровая приставка' : 'Ещё вариант'}
            />

            {/* Первый вариант и есть само желание — убирать его некуда. Место под кнопку
                ему всё равно оставляем, иначе поля разъезжаются по ширине. */}
            {index === 0 && rows.length > 1 && <span className="w-6 shrink-0" aria-hidden />}

            {index > 0 && (
              <button
                type="button"
                onClick={() => setRows((prev) => prev.filter((item) => item.key !== row.key))}
                aria-label={`Убрать вариант ${index + 1}`}
                className="shrink-0 cursor-pointer rounded-sm p-1 text-ink-3 outline-offset-2 hover:text-ink-2 focus-visible:outline-2 focus-visible:outline-brand"
              >
                <IconClose size={16} />
              </button>
            )}
          </div>
        ))}
      </div>

      {suggestions.length > 0 && hasRoom && (
        <div className="flex flex-col gap-2">
          <span className="font-sans text-xs font-semibold text-ink-2">Может подойти</span>

          <div className="flex flex-wrap gap-1.5">
            {suggestions.map((suggestion) => (
              <button
                key={suggestion}
                type="button"
                onClick={() => addSuggestion(suggestion)}
                className="inline-flex cursor-pointer items-center gap-1 rounded-chip bg-brand-soft px-2.5 py-1.5 font-sans text-[12.5px] font-semibold text-brand outline-offset-2 transition-colors hover:bg-brand hover:text-on-brand focus-visible:outline-2 focus-visible:outline-brand"
              >
                <IconPlus size={13} />
                {suggestion}
              </button>
            ))}
          </div>

          {/* Честно про источник: за подсказками стоит не модель, а поиски и избранное. */}
          <p className="text-[12px] text-ink-3">Из ваших поисков и избранного</p>
        </div>
      )}

      {rows.length < MAX_VARIANTS && (
        <Button
          variant="ghost"
          onClick={() => setRows((prev) => [...prev, emptyRow(nextKey.current++)])}
          className="self-start px-3 py-2 text-[13.5px]"
        >
          <IconPlus size={15} />
          Добавить вариант
        </Button>
      )}

      <p className="text-[12.5px] leading-4.5 text-ink-3">
        Подойдёт любой из вариантов — каждый это отдельный шанс замкнуть круг. Свою вещь вы отдадите
        одному человеку, а получите от другого; доплат в обмене нет.
      </p>

      {isError && (
        <p role="alert" className="text-[12.5px] font-semibold text-stop">
          Не удалось сохранить желание. Проверьте, что указали хотя бы один вариант.
        </p>
      )}

      <div className="mt-auto pt-2">
        <Button type="submit" fullWidth disabled={isPending || filled.length === 0}>
          {isPending ? pendingLabel : submitLabel}
        </Button>
      </div>
    </form>
  )
}
