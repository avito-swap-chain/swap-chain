import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { leaveReview, userKeys } from '@/entities/user'
import { ActionError, Button, IconStar, Input } from '@/shared/ui'
import { cx, instrumental } from '@/shared/lib'

const RATINGS = [1, 2, 3, 4, 5]

interface LeaveReviewProps {
  chainId: string
  /** Кого оцениваем: бэкенд принимает отзыв только на прямого соседа по кругу. */
  target: { userId: string; name: string }
}

/**
 * Отзыв после завершённого обмена. Оценка обязательна, текст — нет: написать пару слов
 * готов не каждый, а звезда без слов всё равно говорит больше, чем молчание. Оцениваем
 * того, от кого получили вещь: судить можно о человеке, с которым имел дело, а не обо всех
 * участниках круга.
 */
export function LeaveReview({ chainId, target }: LeaveReviewProps) {
  const [rating, setRating] = useState(0)
  const [text, setText] = useState('')
  const queryClient = useQueryClient()

  const { mutate, isPending, isSuccess, error } = useMutation({
    mutationFn: () => leaveReview(chainId, { targetUserId: target.userId, rating, text }),
    onSuccess: () => {
      // Профиль соседа и его отзывы: рейтинг пересчитан на бэкенде, и старое число
      // в кэше — прямое враньё на экране, куда человек сейчас и пойдёт смотреть.
      queryClient.invalidateQueries({ queryKey: userKeys.all })
    },
  })

  if (isSuccess) {
    return (
      <p className="text-[13px] text-ink-2">
        Спасибо, отзыв опубликован — он виден в профиле участника.
      </p>
    )
  }

  return (
    <section className="flex flex-col gap-2.5 rounded-lg bg-surface-2 p-3">
      <h2 className="text-[15px] font-bold">Как прошёл обмен с {instrumental(target.name)}?</h2>

      <div className="flex gap-1" role="radiogroup" aria-label="Оценка обмена">
        {RATINGS.map((value) => (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={rating === value}
            aria-label={`${value} из 5`}
            onClick={() => setRating(value)}
            className={cx(
              'rounded-sm p-0.5 outline-offset-2 focus-visible:outline-2 focus-visible:outline-brand',
              value <= rating ? 'text-attention' : 'text-ink-3',
            )}
          >
            <IconStar size={26} />
          </button>
        ))}
      </div>

      <Input
        value={text}
        onChange={(event) => setText(event.target.value)}
        placeholder="Пара слов о человеке — по желанию"
        maxLength={500}
      />

      <Button fullWidth disabled={rating === 0 || isPending} onClick={() => mutate()}>
        {isPending ? 'Отправляем…' : 'Оставить отзыв'}
      </Button>

      {/* 409 — отзыв этому человеку по этой цепочке уже оставлен: бэкенд разрешает один. */}
      <ActionError error={error} conflict="Вы уже оставляли отзыв по этому обмену" />
    </section>
  )
}
