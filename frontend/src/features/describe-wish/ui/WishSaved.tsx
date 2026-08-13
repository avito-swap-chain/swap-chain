import { useNavigate } from 'react-router-dom'
import { Button, IconCheck } from '@/shared/ui'

interface WishSavedProps {
  /** Что именно произошло: включили обмен у объявления или опубликовали новое. */
  title: string
}

/**
 * Желание сохранено — вещь ушла в подбор. Экран нужен потому, что дальше ничего видимого
 * не происходит: цепочка собирается на бэкенде и может собраться через час. Раньше человека
 * молча выбрасывало в список, и самый важный момент — «заявка принята» — проходил незамеченным.
 *
 * Здесь же снимается главный страх обмена: вещь не изымают и не блокируют, объявление
 * продолжает работать как обычное.
 */
export function WishSaved({ title }: WishSavedProps) {
  const navigate = useNavigate()

  return (
    <div className="flex flex-1 flex-col items-center gap-4 py-12 text-center">
      <div className="flex size-16 items-center justify-center rounded-full bg-ok-bg text-ok">
        <IconCheck size={28} />
      </div>

      <h2 className="text-[21px] leading-7 font-bold">{title}</h2>

      <p className="max-w-88 text-[13.5px] leading-5 text-ink-2">
        Ищем круг: кому нужна ваша вещь, у кого есть то, что нужно вам. Как только цепочка
        соберётся, придёт уведомление — согласиться или отказаться вы успеете.
      </p>

      <p className="max-w-88 text-[13.5px] leading-5 text-ink-2">
        Вещь остаётся у вас, а объявление — в продаже. Обмен обычной сделке не мешает.
      </p>

      <div className="mt-auto flex w-full flex-col gap-2 pt-8">
        <Button fullWidth onClick={() => navigate('/exchange')}>
          Посмотреть обмены
        </Button>

        <Button variant="ghost" fullWidth onClick={() => navigate('/')}>
          К моим объявлениям
        </Button>
      </div>
    </div>
  )
}
