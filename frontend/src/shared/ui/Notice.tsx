import type { ReactNode } from 'react'
import { cx } from '../lib'

/** Короткое сообщение вместо содержимого: загрузка, ошибка, пустой список. */
export function Notice({
  tone = 'muted',
  children,
}: {
  tone?: 'muted' | 'error'
  children: ReactNode
}) {
  return (
    <p className={cx('py-8 text-center text-sm', tone === 'error' ? 'text-stop' : 'text-ink-2')}>
      {children}
    </p>
  )
}
