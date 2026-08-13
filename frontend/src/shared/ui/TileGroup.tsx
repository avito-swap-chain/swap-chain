import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { cx } from '../lib'

/**
 * Группа строк на серой подложке — главный примитив мобильного кабинета Авито: так собраны
 * «Инструменты», «Ваши данные», «Отзывы». Строки внутри разделены светлой линией, а вся
 * группа читается одним блоком, поэтому у неё скруглены только внешние углы.
 */
export function TileGroup({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cx('overflow-hidden rounded-2xl bg-line-2', className)}>{children}</div>
}

interface TileRowProps {
  icon?: ReactNode
  children: ReactNode
  /** Что стоит справа: счётчик, значение, пометка «Новое». */
  trailing?: ReactNode
  /** Куда ведёт строка. Без `to` и `onClick` строка не нажимается — это просто пункт. */
  to?: string
  onClick?: () => void
}

const ROW =
  'flex w-full items-center gap-3 px-4 py-3.5 text-left text-[15px] leading-5 text-ink not-first:border-t not-first:border-card outline-offset-[-2px] focus-visible:outline-2 focus-visible:outline-brand'
const TAPPABLE = 'cursor-pointer transition-colors hover:bg-line'

export function TileRow({ icon, children, trailing, to, onClick }: TileRowProps) {
  const content = (
    <>
      {icon && <span className="shrink-0 text-ink-2">{icon}</span>}
      <span className="min-w-0 flex-1 truncate">{children}</span>
      {trailing}
    </>
  )

  if (to) {
    return (
      <Link to={to} className={cx(ROW, TAPPABLE)}>
        {content}
      </Link>
    )
  }

  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={cx(ROW, TAPPABLE)}>
        {content}
      </button>
    )
  }

  return <div className={ROW}>{content}</div>
}
