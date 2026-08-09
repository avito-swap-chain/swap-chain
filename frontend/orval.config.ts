import { defineConfig } from 'orval'

/**
 * Клиент к бэкенду генерируется из общего контракта, а не пишется руками:
 * `api/openapi.yaml` — копия `api/openapi.yaml` монорепо (обновляется командой `pnpm api:sync`).
 *
 * Генерируем типы и функции-запросы, но не хуки: серверное состояние в проекте описано
 * своими ключами кэша в api-слое сущностей, и вторая, параллельная система хуков только
 * запутала бы — сгенерированные функции вызываются изнутри наших.
 */
export default defineConfig({
  swapChain: {
    input: './api/openapi.yaml',
    output: {
      target: './src/shared/api/generated/endpoints.ts',
      schemas: './src/shared/api/generated/model',
      client: 'fetch',
      baseUrl: '',
      override: {
        mutator: {
          path: './src/shared/api/fetcher.ts',
          name: 'apiFetch',
        },
      },
    },
  },
})
