import type { ButtonHTMLAttributes } from 'react'
import { cx } from '../lib'

type Variant = 'primary' | 'dark' | 'secondary' | 'ghost' | 'danger'

const base =
  'inline-flex items-center justify-center gap-2 font-sans font-bold text-[15px] px-4 py-[13px] rounded-btn border border-transparent cursor-pointer transition-colors disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand'

const variants: Record<Variant, string> = {
  primary: 'bg-brand text-on-brand hover:bg-brand-press',
  dark: 'bg-dark text-white hover:bg-dark-press',
  secondary: 'bg-line-2 text-ink hover:bg-line',
  ghost: 'bg-transparent text-ink border-line hover:bg-line-2',
  danger: 'bg-transparent text-stop border-stop/40 hover:bg-stop-bg',
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  fullWidth?: boolean
}

export function Button({
  variant = 'primary',
  fullWidth = false,
  type = 'button',
  className,
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cx(base, variants[variant], fullWidth && 'w-full', className)}
      {...rest}
    />
  )
}
