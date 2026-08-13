import type { InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from 'react'
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

/**
 * Вид поля. `card` — обычное поле формы: белое, с рамкой. `soft` — поле на серой заливке
 * без рамки, как на входе в мобильном вебе Авито: там форма из двух полей, и рамки вокруг
 * них на пустом экране выглядят как решётка.
 */
type InputTone = 'card' | 'soft'

type InputProps = InputHTMLAttributes<HTMLInputElement> & { tone?: InputTone }

/* размер `m` Авито: кегль 15/20, фокус — голубая рамка, а не аутлайн */
const CONTROL =
  'w-full rounded-input px-[15px] font-sans text-[15px] leading-5 font-semibold text-ink placeholder:font-normal placeholder:text-ink-3 focus-visible:outline-none'

const TONES: Record<InputTone, string> = {
  card: 'border border-line bg-card focus-visible:border-focus',
  soft: 'border border-transparent bg-line-2 focus-visible:border-focus',
}

export function Input({ className, tone = 'card', ...rest }: InputProps) {
  return <input className={cx(CONTROL, TONES[tone], 'h-11', className)} {...rest} />
}

type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement>

/** Многострочное поле: то же поле ввода, но растёт под текст, а не режет его в одну строку. */
export function Textarea({ className, ...rest }: TextareaProps) {
  return <textarea className={cx(CONTROL, TONES.card, 'resize-none py-2.5', className)} {...rest} />
}
