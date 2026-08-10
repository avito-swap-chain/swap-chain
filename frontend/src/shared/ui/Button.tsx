import type { ButtonHTMLAttributes } from 'react'
import { cx } from '../lib'

type Variant = 'primary' | 'dark' | 'secondary' | 'ghost' | 'danger'

/* Геометрия — размер `m` дизайн-системы Авито: высота 44, кегль 15/20,
 * несимметричные боковые отступы (component-button-size-m-padding: 0 17px 0 16px).
 * Фокус у Авито — кольцо тенью (focusBoxShadow-default), а не аутлайн. */
const base =
  'inline-flex h-11 items-center justify-center gap-2 font-sans font-bold text-[15px] leading-5 pl-4 pr-[17px] rounded-btn border border-transparent cursor-pointer transition-colors disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:shadow-[0_0_0_3px_var(--color-focus)]'

const variants: Record<Variant, string> = {
  primary: 'bg-brand text-on-brand hover:bg-brand-hover active:bg-brand-press',
  dark: 'bg-dark text-white hover:bg-dark-press',
  secondary: 'bg-line-2 text-ink hover:bg-line active:bg-[#e3e2e1]',
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
