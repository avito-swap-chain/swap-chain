import { Outlet, useLocation } from 'react-router-dom'
import { Breadcrumbs } from './Breadcrumbs'
import { CabinetNav } from './CabinetNav'
import { MobileTabBar } from './MobileTabBar'
import { ProfileSummary } from './ProfileSummary'
import { ShellHeader } from './ShellHeader'
import { getSection } from '../lib/navigation'

/**
 * Оболочка «мы внутри Авито»: обмен подаётся не отдельным продуктом, а разделом
 * личного кабинета. Layout-роут — чистая обвязка вокруг `<Outlet />`, бизнес-слои
 * о ней не знают, поэтому раздел встраивается и как отдельное SPA.
 *
 * Раскладка повторяет обе вёрстки Авито, а они у него разные. На узких окнах — мобильная:
 * одна колонка поверх белого листа и нижняя панель разделов, как в их приложении.
 * От `lg` — десктопный кабинет: серая страница, слева колонка с профилем и разделами.
 * Крошки остаются только на десктопе: на телефоне путь назад показывает заголовок экрана.
 */
export function AvitoShell() {
  const { pathname } = useLocation()
  const section = getSection(pathname)

  return (
    <div className="flex min-h-svh flex-col bg-card lg:bg-page">
      <ShellHeader />

      {/* Отступ снизу — под нижнюю панель: иначе она накрывает конец списка. */}
      {/* Отступ сверху — как у Авито: контент кабинета начинается примерно на 150px от верха окна. */}
      <div className="mx-auto flex w-full max-w-page flex-1 flex-col max-lg:pb-[52px] lg:flex-row lg:gap-14 lg:px-6 lg:pt-12 lg:pb-6">
        {/* Левая колонка кабинета Авито: сначала «кто я», под чертой — разделы.
            На узких окнах её роль играет нижняя панель, поэтому колонка целиком скрыта. */}
        <aside className="max-lg:hidden lg:sticky lg:top-6 lg:w-[300px] lg:shrink-0 lg:self-start">
          <ProfileSummary />
          <CabinetNav section={section} />
        </aside>

        <div className="flex min-w-0 flex-1 flex-col">
          <Breadcrumbs pathname={pathname} className="max-lg:hidden" />
          <Outlet />
        </div>
      </div>

      <MobileTabBar section={section} />
    </div>
  )
}
