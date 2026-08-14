/** Раздел кабинета — единственный верхний уровень навигации: им подсвечивается меню. */
export type Section = 'items' | 'exchange' | 'messages' | 'reviews' | 'notifications' | 'blocked'

/**
 * Раздел по текущему роуту. «Обмен» — это и список сделок (`/exchange`), и открытая
 * цепочка (`/exchange/:id`); «Сообщения» — список переписок и открытый диалог;
 * всё остальное живёт в объявлениях.
 */
export function getSection(pathname: string): Section {
  if (pathname === '/exchange' || pathname.startsWith('/exchange/')) return 'exchange'
  if (pathname === '/messages' || pathname.startsWith('/messages/')) return 'messages'
  if (pathname === '/reviews') return 'reviews'
  // Чёрный список — раздел без пункта в меню: подсвечивать вместо него объявления неправильно.
  if (pathname === '/blocked') return 'blocked'
  if (pathname === '/notifications') return 'notifications'
  return 'items'
}
