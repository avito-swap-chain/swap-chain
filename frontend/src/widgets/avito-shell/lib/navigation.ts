/** Раздел кабинета — единственный верхний уровень навигации: им подсвечивается меню. */
export type Section = 'items' | 'exchange'

export interface Crumb {
  label: string
  /** Без `to` — текущая страница либо корень «Авито»: главной Авито в демо нет. */
  to?: string
}

/**
 * Раздел по текущему роуту. «Обмен» — это и список сделок (`/exchange`),
 * и открытая цепочка (`/exchange/:id`); всё остальное живёт в объявлениях.
 */
export function getSection(pathname: string): Section {
  return pathname === '/exchange' || pathname.startsWith('/exchange/') ? 'exchange' : 'items'
}

const AVITO: Crumb = { label: 'Авито' }
const ITEMS: Crumb = { label: 'Мои объявления', to: '/' }
const EXCHANGE: Crumb = { label: 'Обмен', to: '/exchange' }

/**
 * Крошки — только на вложенных экранах: на верхнем уровне их работу делает меню
 * кабинета, и повторять его строкой ниже незачем. Пустой массив = крошек нет.
 */
export function getBreadcrumbs(pathname: string): Crumb[] {
  if (pathname === '/items/new') return [AVITO, ITEMS, { label: 'Новое объявление' }]
  if (/^\/items\/[^/]+\/barter$/.test(pathname)) return [AVITO, ITEMS, { label: 'Готов обменять' }]
  if (/^\/exchange\/[^/]+$/.test(pathname)) return [AVITO, EXCHANGE, { label: 'Цепочка' }]
  return []
}
