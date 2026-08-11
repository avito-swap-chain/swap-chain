import { useRef, useState, type SubmitEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CATEGORIES, itemKeys, suggestWish, type GivenItem, type Wish } from '@/entities/item'
import { Button, IconClose, IconPlus, Input, Select } from '@/shared/ui'

/** Строка формы: вариант желания + стабильный ключ, чтобы удаление не путало поля React. */
interface WishRow extends Wish {
  key: number
}

/** Больше пяти вариантов человек уже не держит в голове, а форма превращается в простыню. */
const MAX_VARIANTS = 5

const emptyRow = (key: number): WishRow => ({ key, category: CATEGORIES[0], description: '' })

/** Первый вариант обязателен, поэтому он и есть само желание; остальные — добавка к нему. */
const rowLabel = (index: number) => (index === 0 ? 'Что хотите взамен' : `Вариант ${index + 1}`)

interface DescribeWishFormProps {
  /** Что человек отдаёт: название показываем, категорию используем для подсказок. */
  give: GivenItem
  /**
   * Уже сохранённое желание. При правке форма открывается с ним: иначе «поменять один
   * вариант» означало бы набрать заново все, и человек терял бы то, что уже указал.
   */
  initial?: Wish[]
  submitLabel: string
  pendingLabel: string
  /**
   * Как сохранить желание. Форма одна на оба входа в сервис — публикацию новой вещи
   * и включение обмена у уже размещённого объявления; отличается только сохранение.
   */
  onSubmit: (wish: Wish[]) => Promise<unknown>
  onDone: () => void
}

/**
 * Желание: что пользователь хочет получить взамен. Вариантов может быть несколько, и это
 * не украшение — каждый вариант отдельное ребро графа и отдельный шанс замкнуть цикл.
 * Широту даём количеством конкретных вариантов, а не размытой формулировкой в одном поле.
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
    initial?.length ? initial.map((wish, index) => ({ ...wish, key: index })) : [emptyRow(0)],
  )
  // Ключи новых строк продолжают занятые: совпадение спутало бы React поля местами.
  const nextKey = useRef(rows.length)
  const queryClient = useQueryClient()

  // Пустые строки не сохраняем: добавить вариант и передумать — нормальный ход.
  const filled = rows
    .map(({ category, description }) => ({ category, description: description.trim() }))
    .filter((wish) => wish.description !== '')

  // Подсказки желания — из поисков и избранного человека, см. `suggestWish`. Уже выбранные
  // варианты уходят из подсказок, поэтому они и лежат в ключе запроса.
  const { data: suggestions = [] } = useQuery({
    queryKey: itemKeys.suggestions(
      give.title,
      filled.map((wish) => wish.description),
    ),
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

  const patch = (key: number, part: Partial<Wish>) =>
    setRows((prev) => prev.map((row) => (row.key === key ? { ...row, ...part } : row)))

  // Есть куда положить ещё один вариант: свободная строка или место под новую.
  const hasRoom = rows.length < MAX_VARIANTS || rows.some((row) => row.description.trim() === '')

  /** Подсказка занимает первую пустую строку, а если пустых нет — становится новым вариантом. */
  const addSuggestion = (suggestion: Wish) => {
    const key = nextKey.current++

    setRows((prev) => {
      const empty = prev.find((row) => row.description.trim() === '')
      if (empty) return prev.map((row) => (row.key === empty.key ? { ...row, ...suggestion } : row))

      return prev.length < MAX_VARIANTS ? [...prev, { ...suggestion, key }] : prev
    })
  }

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    mutate()
  }

  return (
    <form onSubmit={submit} className="flex flex-1 flex-col gap-3.5">
      <p className="text-[13.5px] text-ink-2">
        Отдаёте: <b className="font-bold text-ink">{give.title}</b>
      </p>

      {suggestions.length > 0 && hasRoom && (
        <div className="flex flex-col gap-2 rounded-card border border-line bg-card p-3">
          <span className="font-sans text-xs font-semibold text-ink-2">Может подойти</span>

          <div className="flex flex-wrap gap-1.5">
            {suggestions.map((suggestion) => (
              <button
                key={suggestion.description}
                type="button"
                onClick={() => addSuggestion(suggestion)}
                className="inline-flex cursor-pointer items-center gap-1 rounded-chip bg-brand-soft px-2.5 py-1.5 font-sans text-[12.5px] font-semibold text-brand outline-offset-2 transition-colors hover:bg-brand hover:text-on-brand focus-visible:outline-2 focus-visible:outline-brand"
              >
                <IconPlus size={13} />
                {suggestion.description}
              </button>
            ))}
          </div>

          <p className="text-[12px] text-ink-3">Из ваших поисков и избранного</p>
        </div>
      )}

      <div className="flex flex-col gap-3">
        {rows.map((row, index) => (
          <div key={row.key} className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between gap-2">
              <span className="font-sans text-xs font-semibold text-ink-2">{rowLabel(index)}</span>

              {index > 0 && (
                <button
                  type="button"
                  onClick={() => setRows((prev) => prev.filter((item) => item.key !== row.key))}
                  aria-label={`Убрать вариант ${index + 1}`}
                  className="cursor-pointer rounded-sm p-0.5 text-ink-3 outline-offset-2 hover:text-ink-2 focus-visible:outline-2 focus-visible:outline-brand"
                >
                  <IconClose size={15} />
                </button>
              )}
            </div>

            {/* Категория и описание в одну строку, как в поиске Авито: «где искать» + «что». */}
            <div className="grid grid-cols-[38%_1fr] gap-2">
              <Select
                value={row.category}
                onChange={(event) => patch(row.key, { category: event.target.value })}
                aria-label={`Категория варианта ${index + 1}`}
                className="px-2.5 text-[13px]"
              >
                {CATEGORIES.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </Select>

              <Input
                value={row.description}
                onChange={(event) => patch(row.key, { description: event.target.value })}
                aria-label={rowLabel(index)}
                placeholder={index === 0 ? 'Например, игровая приставка' : 'Ещё вариант'}
              />
            </div>
          </div>
        ))}
      </div>

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

      <p className="text-[12.5px] text-ink-3">
        Подойдёт любой из вариантов — чем их больше, тем выше шанс найти цепочку.
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
