import { Fragment } from 'react'
import { Link } from 'react-router-dom'
import { cx } from '@/shared/lib'
import { getBreadcrumbs } from '../lib/navigation'

/**
 * Крошки считаются из роута — страницы о своём месте в навигации Авито не знают.
 * Есть только на вложенных экранах, а они все узкие, поэтому ширина крошек совпадает
 * с `Screen width="narrow"`. Обе строки прижаты к левому краю колонки, как в кабинете
 * Авито: когда экран центрировали, а крошки нет, они расходились на добрую сотню пикселей.
 */
export function Breadcrumbs({ pathname, className }: { pathname: string; className?: string }) {
  const crumbs = getBreadcrumbs(pathname)
  if (crumbs.length === 0) return null

  return (
    <nav
      aria-label="Хлебные крошки"
      className={cx(
        'flex w-full max-w-2xl items-center gap-1.5 px-4 pt-2 text-[12px] text-ink-3 sm:pt-2.5 lg:px-0 lg:pt-0 lg:pb-2.5',
        className,
      )}
    >
      {crumbs.map((crumb, i) => (
        <Fragment key={crumb.label}>
          {i > 0 && <span aria-hidden="true">›</span>}
          {crumb.to ? (
            <Link
              to={crumb.to}
              className="rounded-sm outline-offset-2 hover:text-ink-2 focus-visible:outline-2 focus-visible:outline-brand"
            >
              {crumb.label}
            </Link>
          ) : (
            <span className={i === crumbs.length - 1 ? 'text-ink-2' : undefined}>
              {crumb.label}
            </span>
          )}
        </Fragment>
      ))}
    </nav>
  )
}
