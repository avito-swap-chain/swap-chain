import { ActionError, Button, IconImage } from '@/shared/ui'

interface RecognitionFailedProps {
  error: unknown
  onRetry: () => void
  onManual: () => void
}

/**
 * Модель не справилась. Отдельный экран, а не строка ошибки над формой: до этого места
 * человек ничего не заполнял — сообщать не о чем, кроме самого отказа, и решать ему одно:
 * снять заново или писать руками. Второй выход обязателен — без него отказ модели
 * закрывал бы дорогу к публикации вовсе.
 */
export function RecognitionFailed({ error, onRetry, onManual }: RecognitionFailedProps) {
  return (
    <div className="flex flex-1 flex-col items-center gap-4 py-12 text-center">
      <div className="flex size-20 items-center justify-center rounded-full bg-line-2 text-ink-3">
        <IconImage size={34} />
      </div>

      <h2 className="text-[19px] leading-6 font-bold">Не смогли рассмотреть вещь как следует</h2>

      <p className="max-w-80 text-[13.5px] leading-5 text-ink-2">
        Снимите вещь целиком и при хорошем свете — или заполните поля сами, это займёт минуту.
      </p>

      <ActionError error={error} conflict="Это фото распознать не получилось" />

      <div className="mt-auto flex w-full flex-col gap-2 pt-6">
        <Button fullWidth onClick={onRetry}>
          Сфотографировать заново
        </Button>

        <Button variant="ghost" fullWidth onClick={onManual}>
          Заполнить самому
        </Button>
      </div>
    </div>
  )
}
