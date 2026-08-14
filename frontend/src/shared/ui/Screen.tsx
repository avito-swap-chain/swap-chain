import type { ReactNode } from 'react'
import { cx } from '../lib'
import { IconChevronLeft } from './icons'

/**
 * Ширина контентной колонки:
 * - `narrow` — экран одного действия (форма, цепочка): за окном не растёт, читается как лист;
 * - `wide` — списки: занимают всю ширину контента, внутри раскладываются сеткой.
 */
export type ScreenWidth = 'narrow' | 'wide'

interface ScreenProps {
  width?: ScreenWidth
  children: ReactNode
}

/**
 * Каркас экрана. Высоту задаёт оболочка (`widgets/avito-shell`), экран растягивается
 * в остаток — иначе шапка Авито и экран сложились бы в два экрана высоты.
 *
 * Белый лист во всю ширину — и на телефоне, и на десктопе: страницы кабинета у Авито
 * не карточки на серой подложке, содержимое лежит прямо на листе. Узкий экран просто
 * ограничен по ширине, чтобы форма не растягивалась на всю страницу.
 */
export function Screen({ width = 'narrow', children }: ScreenProps) {
  return (
    <div
      className={cx(
        'flex w-full flex-1 flex-col bg-card',
        width === 'narrow' && 'max-w-2xl lg:flex-none',
      )}
    >
      {children}
    </div>
  )
}

interface ScreenHeaderProps {
  title: ReactNode
  /** Если передан — слева появляется кнопка «Назад». */
  onBack?: () => void
  /** Правый слот шапки: статус-чип, аватар и подобное. */
  children?: ReactNode
}

export function ScreenHeader({ title, onBack, children }: ScreenHeaderProps) {
  return (
    <header className="flex items-center gap-2.5 border-b border-line-2 px-4 py-3 sm:py-3.5">
      {onBack && (
        <button
          type="button"
          onClick={onBack}
          aria-label="Назад"
          className="cursor-pointer rounded-sm text-ink-2 outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand"
        >
          <IconChevronLeft size={19} />
        </button>
      )}
      <h1 className="flex-1 text-[16px] font-bold">{title}</h1>
      {children}
    </header>
  )
}
