import { cx } from '@/shared/lib'
import type { DeliveryStatus } from '../model/types'

/**
 * Путь вещи через пункт выдачи: приняли → повезли → выдали. Три шага, потому что ровно
 * столько раз вещь меняет место, и каждый шаг — физическое действие сотрудника.
 *
 * Нужен именно за стойкой: по одной кнопке «Отправить получателю» не видно, на каком
 * отрезке круга стоит эта вещь и сколько ей ещё ехать. Горизонтальный, а не `Steps`:
 * тот вертикальный и рассчитан на отдельный экран, а здесь строка внутри цепочки.
 */
const STEPS = ['ПВЗ', 'Доставка', 'Получение'] as const

/** Сколько шагов пути уже пройдено. `AWAITING_PVZ` — вещь ещё несут, пройденного нет. */
const DONE: Record<DeliveryStatus, number> = {
  AWAITING_PVZ: 0,
  AT_PVZ: 1,
  IN_DELIVERY: 2,
  RECEIVED: 3,
}

export function DeliveryProgress({ status }: { status: DeliveryStatus }) {
  const done = DONE[status]

  return (
    <ol className="flex items-center gap-1" aria-label="Путь вещи через пункт выдачи">
      {STEPS.map((label, index) => {
        const passed = index < done
        // Текущий — первый непройденный: именно он ждёт действия сотрудника.
        const current = index === done

        return (
          <li key={label} className="flex items-center gap-1">
            {index > 0 && (
              <span aria-hidden className={cx('h-px w-4', passed ? 'bg-ink-3' : 'bg-line')} />
            )}
            <span
              aria-current={current ? 'step' : undefined}
              className={cx(
                'flex items-center gap-1.5 text-[12.5px] leading-4',
                passed && 'text-ink-2',
                current && 'font-bold text-ink',
                !passed && !current && 'text-ink-3',
              )}
            >
              <span
                aria-hidden
                className={cx(
                  'size-2 rounded-full',
                  passed && 'bg-ink-3',
                  current && 'bg-attention-dot',
                  !passed && !current && 'bg-line',
                )}
              />
              {label}
            </span>
          </li>
        )
      })}
    </ol>
  )
}
