import type { ReactNode } from 'react'
import { cx } from '../lib'

interface TileProps {
  /** Крупная строка: число, рейтинг со звёздами. */
  value: ReactNode
  /** Что это за число — мелким серым под ним. */
  label: ReactNode
  className?: string
}

/**
 * Плитка-метрика мобильного кабинета Авито: серая подложка, крупное значение и подпись.
 * Стоят по две в ряд — рейтинг и уровень сервиса; у нас рейтинг и завершённые обмены.
 */
export function Tile({ value, label, className }: TileProps) {
  return (
    <div className={cx('flex flex-col gap-1 rounded-2xl bg-line-2 px-4 py-3.5', className)}>
      <span className="flex items-center gap-1.5 text-[19px] leading-6 font-bold">{value}</span>
      <span className="text-[13px] leading-4 text-ink-2">{label}</span>
    </div>
  )
}
