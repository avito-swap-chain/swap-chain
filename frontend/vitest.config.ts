import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

// Отдельно от vite.config.ts: юнит-тесты покрывают чистую логику,
// плагины react/tailwind им не нужны — нужен только алиас `@`.
export default defineConfig({
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  test: {
    // `.tsx` — тесты экранов. Окружение им нужно браузерное, но включается оно построчно
    // (`// @vitest-environment jsdom`) в самих файлах: логических тестов на порядок больше,
    // и поднимать им jsdom не за что.
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
})
