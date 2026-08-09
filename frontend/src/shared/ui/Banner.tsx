import type { ReactNode } from 'react'
import { cx } from '../lib'

type Tone = 'info' | 'ok' | 'stop'

const tones: Record<Tone, string> = {
  info: 'bg-brand-soft [&_svg]:text-brand',
  ok: 'bg-ok-bg [&_svg]:text-ok',
  stop: 'bg-stop-bg [&_svg]:text-stop',
}

interface BannerProps {
  tone?: Tone
  icon?: ReactNode
  children: ReactNode
  className?: string
}

export function Banner({ tone = 'info', icon, children, className }: BannerProps) {
  return (
    <div
      className={cx(
        'flex items-center gap-[11px] rounded-2xl px-[14px] py-[13px] text-[13.5px] text-ink',
        tones[tone],
        className,
      )}
    >
      {icon}
      <span>{children}</span>
    </div>
  )
}
