import type { ReactNode } from 'react'
import { cx } from '../lib'

/**
 * Статус — это текст, а не плашка: так набраны статусы заказов у Авито.
 * Смысл несёт цвет: `attention` — ход за пользователем, `stop` — проблема,
 * `ok` — сделано как надо, `neutral` — обычное состояние, `muted` — то,
 * что уже никуда не движется.
 */
export type StatusTone = 'neutral' | 'attention' | 'ok' | 'stop' | 'muted'

const tones: Record<StatusTone, string> = {
  neutral: 'text-ink',
  attention: 'text-attention',
  ok: 'text-ok',
  stop: 'text-stop',
  muted: 'text-ink-3',
}

interface StatusProps {
  tone?: StatusTone
  children: ReactNode
  className?: string
}

export function Status({ tone = 'neutral', children, className }: StatusProps) {
  return (
    <span
      className={cx(
        /* размер `s` шкалы Авито (13/16); 12px в их шкале кегля нет */
        'font-sans text-[13px] leading-4 font-semibold whitespace-nowrap',
        tones[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}
