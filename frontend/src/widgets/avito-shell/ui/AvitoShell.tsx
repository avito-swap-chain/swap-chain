import { Outlet, useLocation } from 'react-router-dom'
import { cx } from '@/shared/lib'
import { Breadcrumbs } from './Breadcrumbs'
import { CabinetNav } from './CabinetNav'
import { ShellHeader } from './ShellHeader'
import { getBreadcrumbs, getSection } from '../lib/navigation'

/**
 * Оболочка «мы внутри Авито»: обмен подаётся не отдельным продуктом, а разделом
 * личного кабинета. Layout-роут — чистая обвязка вокруг `<Outlet />`, бизнес-слои
 * о ней не знают, поэтому раздел встраивается и как отдельное SPA.
 *
 * Раскладка: на узких окнах всё идёт одной колонкой поверх белого листа,
 * от `lg` меню кабинета уходит в левую колонку, а фоном становится серая страница.
 *
 * На вложенных экранах (цепочка, формы) мобильная оболочка сворачивается: меню разделов
 * и крошки прячутся, остаётся шапка и заголовок страницы с кнопкой «назад». Иначе на
 * экране 360×640 четыре яруса навигации съедали треть высоты, а крошки повторяли то же,
 * что и кнопка возврата. На десктопе места хватает — там показываем всё.
 */
export function AvitoShell() {
  const { pathname } = useLocation()
  const section = getSection(pathname)
  const nested = getBreadcrumbs(pathname).length > 0

  return (
    <div className="flex min-h-svh flex-col bg-card lg:bg-page">
      <ShellHeader />

      <div className="mx-auto flex w-full max-w-page flex-1 flex-col lg:flex-row lg:gap-6 lg:px-6 lg:py-6">
        <CabinetNav section={section} className={cx(nested && 'max-lg:hidden')} />

        <div className="flex min-w-0 flex-1 flex-col">
          <Breadcrumbs pathname={pathname} className="max-lg:hidden" />
          <Outlet />
        </div>
      </div>
    </div>
  )
}
