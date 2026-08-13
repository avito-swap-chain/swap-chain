import { useEffect, useState } from 'react'
import type { RecognitionStage } from '@/entities/item'
import { Button, IconSparkle } from '@/shared/ui'

/**
 * Что происходит прямо сейчас. Подписи привязаны к настоящим шагам разбора, а не к таймеру:
 * фото правда сначала уезжает в хранилище, потом устанавливается поток событий, и только
 * после этого за снимок берётся модель.
 */
const STAGE_LABEL: Record<RecognitionStage, string> = {
  upload: 'Загружаем фото',
  connect: 'Подключаем нейросеть',
  analyze: 'Модель разбирает фото',
}

/** Когда предлагать выход. Раньше — суетливо, позже — человек уже решил, что всё зависло. */
const CANCEL_AFTER_MS = 5_000

/** Когда признать, что ответ идёт дольше обычного. Молчащий экран к этому моменту пугает. */
const LONG_AFTER_MS = 12_000

interface RecognitionProgressProps {
  stage: RecognitionStage
  onCancel: () => void
}

export function RecognitionProgress({ stage, onCancel }: RecognitionProgressProps) {
  const waited = useWaited()

  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-6 py-16">
      <div className="relative flex size-16 items-center justify-center">
        <IconSparkle
          size={34}
          className="animate-pulse text-accent-green motion-reduce:animate-none"
        />
        <IconSparkle
          size={19}
          className="absolute top-1 right-1 animate-pulse text-brand [animation-delay:0.35s] motion-reduce:animate-none"
        />
        <IconSparkle
          size={13}
          className="absolute right-3 bottom-2 animate-pulse text-accent-red [animation-delay:0.7s] motion-reduce:animate-none"
        />
      </div>

      {/* Подпись меняется по ходу разбора, поэтому её проговаривает и экранный читатель. */}
      <p aria-live="polite" className="text-[15px] leading-5 font-semibold">
        {waited >= LONG_AFTER_MS ? 'Ещё пара секунд' : STAGE_LABEL[stage]}
      </p>

      {waited >= CANCEL_AFTER_MS && (
        <Button variant="secondary" onClick={onCancel} className="min-w-40">
          Отменить
        </Button>
      )}
    </div>
  )
}

/** Сколько экран уже ждёт — по нему решается, пора ли предлагать выход. */
function useWaited(): number {
  const [waited, setWaited] = useState(0)

  useEffect(() => {
    const timers = [CANCEL_AFTER_MS, LONG_AFTER_MS].map((at) => setTimeout(() => setWaited(at), at))

    return () => timers.forEach(clearTimeout)
  }, [])

  return waited
}
