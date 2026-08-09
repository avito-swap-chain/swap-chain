import type { InputHTMLAttributes, ReactNode } from 'react'
import { cx } from '../lib'

interface FieldProps {
  label?: string
  children: ReactNode
  className?: string
}

export function Field({ label, children, className }: FieldProps) {
  return (
    <label className={cx('flex flex-col gap-1.5', className)}>
      {label && <span className="font-sans text-xs font-semibold text-ink-2">{label}</span>}
      {children}
    </label>
  )
}

type InputProps = InputHTMLAttributes<HTMLInputElement>

export function Input({ className, ...rest }: InputProps) {
  return (
    <input
      className={cx(
        'w-full rounded-input border border-line bg-card px-[13px] py-3 font-sans text-[14.5px] font-semibold text-ink placeholder:font-normal placeholder:text-ink-3 focus-visible:outline-2 focus-visible:outline-brand',
        className,
      )}
      {...rest}
    />
  )
}
