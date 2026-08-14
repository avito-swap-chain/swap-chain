import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { categoryKeys, getCategories, itemKeys, resolveWishCategories } from '@/entities/item'
import { ActionError, Button, Select } from '@/shared/ui'

interface ResolveWishCategoryProps {
  itemId: string
  /** Варианты желания, категорию которых модель не определила. */
  wishes: { id: number; description: string }[]
}

/**
 * Выбрать категорию для желаний, в которых не разобралась модель. Пока она не выбрана,
 * вещь стоит вне подбора, поэтому спрашиваем прямо на карточке, а не прячем в правку
 * объявления: человек пришёл в кабинет и должен увидеть, что от него чего-то ждут.
 *
 * Раскрывается по кнопке: нерешённых вариантов бывает несколько, и раскрытый список
 * селектов в каждой карточке превратил бы кабинет в анкету.
 */
export function ResolveWishCategory({ itemId, wishes }: ResolveWishCategoryProps) {
  const [open, setOpen] = useState(false)
  const [chosen, setChosen] = useState<Record<number, number>>({})
  const queryClient = useQueryClient()

  // Тот же справочник, по которому идёт подбор: выбирать из своего списка значит
  // отправить раздел, которого у бэкенда нет.
  const { data: categories = [] } = useQuery({
    queryKey: categoryKeys.list(),
    queryFn: getCategories,
    enabled: open,
  })

  const resolve = useMutation({
    mutationFn: () =>
      resolveWishCategories(
        itemId,
        wishes.map((wish) => ({ wishId: wish.id, categoryId: chosen[wish.id] })),
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: itemKeys.my() })
    },
  })

  // Бэкенд применяет решение целиком и одним запросом — половинчатый ответ он отвергнет,
  // поэтому кнопка ждёт раздела для каждого варианта.
  const ready = wishes.every((wish) => chosen[wish.id])

  if (!open) {
    return (
      <Button variant="ghost" onClick={() => setOpen(true)}>
        Выбрать категорию
      </Button>
    )
  }

  return (
    <div className="flex w-full flex-col gap-2">
      {wishes.map((wish) => (
        <label key={wish.id} className="flex flex-col gap-1">
          <span className="text-[12.5px] text-ink-2">
            Хочу «{wish.description}» — это категория
          </span>
          <Select
            value={chosen[wish.id] ?? ''}
            onChange={(event) =>
              setChosen((prev) => ({ ...prev, [wish.id]: Number(event.target.value) }))
            }
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
        </label>
      ))}

      <div className="flex items-center gap-3">
        <Button disabled={!ready || resolve.isPending} onClick={() => resolve.mutate()}>
          Сохранить
        </Button>
        <Button variant="ghost" onClick={() => setOpen(false)}>
          Отмена
        </Button>
      </div>

      <ActionError error={resolve.error} />
    </div>
  )
}
