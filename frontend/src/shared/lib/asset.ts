/**
 * Путь к файлу из `public`. На GitHub Pages сайт живёт в подпапке репозитория,
 * поэтому абсолютный путь вида `/mock/items/bike.jpg` там указывает мимо — в корень
 * домена. Vite подставляет базу в `BASE_URL`, и все ссылки на статику идут через неё.
 *
 * Адреса с внешнего бэкенда (`http://…`) и data-URL остаются как есть.
 */
export function asset(path: string | undefined): string | undefined {
  if (!path || /^(https?:|data:|blob:)/.test(path)) return path

  return `${import.meta.env.BASE_URL}${path.replace(/^\//, '')}`
}
