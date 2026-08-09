import type { HTMLAttributes } from 'react'
import { cx } from '../lib'

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  padded?: boolean
}

export function Card({ padded = false, className, ...rest }: CardProps) {
  return (
    <div
      className={cx(
        /* У Авито карточка в потоке плоская: тень означает всплытие (поповер,
         * липкая панель), а не «карточность». Поэтому только рамка. */
        'rounded-card border border-line bg-card',
        padded && 'p-4',
        className,
      )}
      {...rest}
    />
  )
}
