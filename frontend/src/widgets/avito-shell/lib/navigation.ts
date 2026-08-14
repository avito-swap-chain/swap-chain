/** Раздел кабинета — единственный верхний уровень навигации: им подсвечивается меню. */
export type Section =
  | 'items'
  | 'exchange'
  | 'messages'
  | 'reviews'
  | 'notifications'
  | 'blocked'
  | 'profile'

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
  // Свой профиль — пункт меню; чужой (`/users/:id`) им не подсвечивается, туда приходят
  // из цепочки или переписки, а не из кабинета.
  if (pathname === '/profile') return 'profile'
  return 'items'
}
