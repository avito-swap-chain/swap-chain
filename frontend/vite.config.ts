import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  /**
   * GitHub Pages раздаёт сайт из подпапки с именем репозитория, и путь у личного
   * репозитория отличается от командного. Поэтому база приходит переменной сборки:
   * `BASE_PATH=/avito-swap-chain-frontend/ pnpm build`. Локально и в Docker — корень.
   */
  base: process.env.BASE_PATH ?? '/',
  /**
   * Проверка на живом бэкенде: `API_PROXY=https://стенд VITE_API_URL=/ pnpm dev`.
   * Ходить в стенд напрямую из dev-сервера нельзя: сессия лежит в куке `SameSite=Lax`,
   * и на кросс-сайтовый запрос браузер её не приложит. Прокси делает API тем же origin.
   */
  server: process.env.API_PROXY
    ? { proxy: { '/api': { target: process.env.API_PROXY, changeOrigin: true, secure: false } } }
    : undefined,
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
})
