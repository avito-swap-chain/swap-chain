import type { ReactNode } from 'react'
import { cx } from '../lib'

export type StepState = 'done' | 'current' | 'todo'

export interface Step {
  content: ReactNode
  state: StepState
}

// Оранжевая точка и жирный чёрный текст — только у текущего шага: на него смотрят первым.
// Пройденное набрано спокойно, будущее — светло-серым, как этапы доставки у Авито.
const dot: Record<StepState, string> = {
  done: 'bg-ink-3',
  current: 'bg-attention-dot',
  todo: 'bg-line',
}

const text: Record<StepState, string> = {
  done: 'text-ink-2',
  current: 'font-bold text-ink',
  todo: 'text-ink-3',
}

/**
 * Вертикальный степпер: точки в столбик на пунктирной линии — фирменный способ Авито
 * показывать этапы сделки. Заменяет нумерованный список: номера говорят «сколько всего»,
 * а точки — «где мы сейчас».
 */
export function Steps({ steps, className }: { steps: Step[]; className?: string }) {
  return (
    <ol className={cx('flex flex-col', className)}>
      {steps.map((step, index) => (
        <li
          key={index}
          aria-current={step.state === 'current' ? 'step' : undefined}
          className="relative flex gap-3 pb-4 last:pb-0"
        >
          {/* линия дотягивается до точки следующего шага, поэтому уходит за границу пункта */}
          {index < steps.length - 1 && (
            <span className="absolute top-4 bottom-[-2px] left-[3.5px] border-l border-dashed border-line" />
          )}
          <span className={cx('mt-[5px] size-2 shrink-0 rounded-full', dot[step.state])} />
          <span className={cx('text-[13.5px] leading-[18px]', text[step.state])}>
            {step.content}
          </span>
        </li>
      ))}
    </ol>
  )
}
