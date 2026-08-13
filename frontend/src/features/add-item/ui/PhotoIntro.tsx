import { useRef, type ChangeEvent, type ReactNode } from 'react'
import { Button, IconImage, IconSparkle, IconSwap } from '@/shared/ui'

/**
 * Что человек получит и что от него нужно. Три пункта, а не пять: первый про снимок,
 * второй про границу между моделью и человеком, третий про то, чем обмен отличается
 * от продажи. Последний здесь и стоит — дальше в форме нет ни одного поля про деньги,
 * и без объяснения их отсутствие читается как недоделка.
 */
const HOW: { icon: ReactNode; text: string }[] = [
  {
    icon: <IconImage size={19} />,
    text: 'Снимите вещь целиком и при хорошем свете — так модель разберёт её точнее.',
  },
  {
    icon: <IconSparkle size={17} />,
    text: 'Модель предложит описание, категорию и состояние. Название пишете вы: угадывать бренд она не станет.',
  },
  {
    icon: <IconSwap size={19} />,
    text: 'Цену указывать не нужно. Вещь меняется на вещь, доплат в обмене нет.',
  },
]

interface PhotoIntroProps {
  onPick: (file: File) => void
  /** Без фото тоже можно: фотография ускоряет подбор, но не является условием обмена. */
  onSkip: () => void
}

export function PhotoIntro({ onPick, onSkip }: PhotoIntroProps) {
  const input = useRef<HTMLInputElement>(null)

  const pick = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file) onPick(file)
  }

  return (
    <div className="flex flex-1 flex-col gap-6">
      <div className="flex h-36 items-center justify-center rounded-2xl bg-brand-pale text-brand sm:h-44">
        <IconImage size={52} />
      </div>

      <div className="flex flex-col gap-2">
        <h2 className="text-[21px] leading-7 font-bold">Добавьте вещь по фото</h2>
        <p className="text-[13.5px] leading-5 text-ink-2">
          Модель разберёт снимок и предложит, что написать. Останется вписать название и сказать,
          что хотите взамен.
        </p>
      </div>

      <div className="flex flex-col gap-3">
        <b className="text-[15px] font-bold">Как это работает</b>

        <ul className="flex flex-col gap-3">
          {HOW.map((step) => (
            <li key={step.text} className="flex gap-3">
              <span className="mt-0.5 shrink-0 text-ink-2">{step.icon}</span>
              <span className="text-[13.5px] leading-5 text-ink">{step.text}</span>
            </li>
          ))}
        </ul>
      </div>

      <div className="mt-auto flex flex-col gap-2 pt-2">
        {/* Кнопка вместо подписи-обёртки: у поля выбора файла свой вид, а он не совпадает
            ни с одной кнопкой Авито. */}
        <Button fullWidth onClick={() => input.current?.click()}>
          Сфотографировать вещь
        </Button>

        <Button variant="ghost" fullWidth onClick={onSkip}>
          Заполнить без фото
        </Button>

        <input
          ref={input}
          type="file"
          accept="image/*"
          className="sr-only"
          onChange={pick}
          aria-label="Фотография вещи"
        />
      </div>
    </div>
  )
}
