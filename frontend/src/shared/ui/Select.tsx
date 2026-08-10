import type { SelectHTMLAttributes } from 'react'
import { cx } from '../lib'

type SelectProps = SelectHTMLAttributes<HTMLSelectElement>

/** Нативный select со стилями поля — родная выпадашка удобнее своей на мобильных. */
export function Select({ className, ...rest }: SelectProps) {
  return (
    <select
      className={cx(
        'w-full cursor-pointer rounded-input border border-line bg-card px-[13px] py-3 font-sans text-[14.5px] font-semibold text-ink focus-visible:outline-2 focus-visible:outline-brand',
        className,
      )}
      {...rest}
    />
  )
}
