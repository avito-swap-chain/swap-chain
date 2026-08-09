/** Раздел кабинета — единственный верхний уровень навигации: им подсвечивается меню. */
export type Section = 'items' | 'exchange' | 'messages' | 'notifications'

export interface Crumb {
  label: string
  /** Без `to` — текущая страница либо корень «Авито»: главной Авито в демо нет. */
  to?: string
}

/**
 * Раздел по текущему роуту. «Обмен» — это и список сделок (`/exchange`), и открытая
 * цепочка (`/exchange/:id`); «Сообщения» — список переписок и открытый диалог;
 * всё остальное живёт в объявлениях.
 */
export function getSection(pathname: string): Section {
  if (pathname === '/exchange' || pathname.startsWith('/exchange/')) return 'exchange'
  if (pathname === '/messages' || pathname.startsWith('/messages/')) return 'messages'
  if (pathname === '/notifications') return 'notifications'
  return 'items'
}

const AVITO: Crumb = { label: 'Авито' }
const ITEMS: Crumb = { label: 'Мои объявления', to: '/' }
const EXCHANGE: Crumb = { label: 'Обмен', to: '/exchange' }
const MESSAGES: Crumb = { label: 'Сообщения', to: '/messages' }

/**
 * Крошки — только на вложенных экранах: на верхнем уровне их работу делает меню
 * кабинета, и повторять его строкой ниже незачем. Пустой массив = крошек нет.
 */
export function getBreadcrumbs(pathname: string): Crumb[] {
  if (pathname === '/items/new') return [AVITO, ITEMS, { label: 'Новое объявление' }]
  if (/^\/items\/[^/]+\/barter$/.test(pathname)) return [AVITO, ITEMS, { label: 'Готов обменять' }]
  if (/^\/exchange\/[^/]+$/.test(pathname)) return [AVITO, EXCHANGE, { label: 'Цепочка' }]
  if (/^\/messages\/[^/]+$/.test(pathname)) return [AVITO, MESSAGES, { label: 'Переписка' }]
  return []
}
