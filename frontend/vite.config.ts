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
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
})
