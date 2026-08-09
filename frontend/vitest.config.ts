import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

// Отдельно от vite.config.ts: юнит-тесты покрывают чистую логику,
// плагины react/tailwind им не нужны — нужен только алиас `@`.
export default defineConfig({
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  test: {
    include: ['src/**/*.test.ts'],
  },
})
