import type { HTMLAttributes } from 'react'
import { cx } from '../lib'

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  padded?: boolean
}

export function Card({ padded = false, className, ...rest }: CardProps) {
  return (
    <div
      className={cx(
        'rounded-card border border-line bg-card shadow-card',
        padded && 'p-4',
        className,
      )}
      {...rest}
    />
  )
}
