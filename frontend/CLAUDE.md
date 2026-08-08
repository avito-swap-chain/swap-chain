# avito-chain — frontend «Цепочка обмена»

Клиентская часть MVP сервиса **многостороннего обмена** (Avito Start, Кейс 6). Пользователь
добавляет ненужную вещь и что хочет взамен, сервис строит **цепочку** A→B→C→A (каждый отдаёт своё,
получает нужное), пользователь видит цепочку, подтверждает участие и доводит обмен до конца.
Часть командного монорепо (этот фронт + Go-бэкенд + docker-compose).

## Стек

pnpm · Vite · **React 19** · TypeScript **strict** · **TanStack Query** (серверное состояние) ·
**Zustand** (клиентское) · **Tailwind v4** · oxlint + Prettier · **Feature-Sliced Design**.

## Команды

Пакетный менеджер — **corepack pnpm 11** (закреплён в `packageManager`; не `npm`/`npx`).
`pnpm dev` · `pnpm build` · `pnpm lint` (oxlint) · `pnpm format` (prettier).

**Автопроверки (хуки `.claude/settings.json`):** после каждого Edit/Write Prettier форматит изменённый
`.ts/.tsx/.css`; при завершении работы прогоняется `tsc -b` и блокирует финиш при ошибках типов.
LSP в редакторе иногда ложно не резолвит зависимости — авторитетна сборка `pnpm build`, не LSP.

## Конвенции

- **Стили — только Tailwind-утилиты на токенах Avito.** Токены заданы в `@theme` (`src/index.css`):
  `bg-brand` (#00AAFF), `text-ink`, `border-line`, `rounded-btn`, `shadow-card` и т.д. Хардкод цветов
  не использовать — только через токены. **Тема только светлая** (`color-scheme: light`), как веб Avito.
- **Роли цветов, как у Авито:** азур (`brand`) — только ссылки и кнопки; `attention` (#F26100, на точках
  степпера `attention-dot`) — «ход за пользователем»; `stop` — проблема; `ink-3` — то, что уже не движется.
  Статусы набираются текстом (`shared/ui/Status`), а не пилюлями; заливка — за баннерами и тегами.
- **FSD, импорты только вниз:** `app → pages → widgets → features → entities → shared`. Слайсы одного
  слоя друг друга не импортят; наружу — через `index.ts` (public API). Сегменты: `ui / model / api / lib`.
  Между слоями импорт через алиас `@/` (`@/entities/chain`), внутри слайса — относительный (`../model/types`).
- **`shared/ui` готов:** `BrandMark` (лого-точки), `Button` (primary/dark/secondary/ghost/danger, fullWidth),
  `Status` (neutral/attention/stop/muted — статус текстом, как в заказах Авито), `Steps` (вертикальный
  степпер этапов: done/current/todo), `Card` (padded), `Field`+`Input`, `Banner` (info/ok/stop),
  иконки (`IconCheck`/`IconClock`/`IconClose`/`IconPlus`/`IconArrowRight`/`IconChevronLeft`/`IconBox`).
  Хелпер склейки классов — `shared/lib/cx`. Провайдеры — `app/providers.tsx` (QueryClientProvider).
- `verbatimModuleSyntax` включён — импорт типов только через `import type`.
- **Данные — только через TanStack Query.** Запросы в `entities/*/api`, компоненты берут их через
  `useQuery`/`useMutation`; `fetch` в компонентах не вызывать. Пока бэкенда нет — мок-функции в `api`
  (напр. `entities/item/api/itemsApi`), позже меняются на реальные вызовы Go-API без правок компонентов.
- **Коммиты — Conventional Commits** (`feat:`/`fix:`/`chore:`/`refactor:`), под своим GitHub-аккаунтом
  с настоящим именем (атрибуция вклада — требование хакатона).
- **Тесты — Vitest**, юнит на важной логике (машина состояний цепочки, подбор, статус-маппинги).

## Ключевое архитектурное решение

Экраны «предложение / цепочка / ожидание / завершение / распад» — это **один роут
`pages/exchange/:id`**, который рендерит разное по состоянию: `chain.status × myParticipant.status`
(FORMED → PENDING/CONFIRMED → ACTIVE → COMPLETED / DISSOLVED). Виджет `chain-board` выбирает под-вид.
Код повторяет машину состояний сделки 1:1.

Доменные сущности: `item` (вещь + желание), `chain` (цепочка + участники + состояние; участники
денормализованы, чтобы не импортить `entities/user`). AI-автозаполнение вещи по фото — в
`entities/item/api/recognizeItem`, дёргается из `features/add-item`.

## Требования кейса (влияют на код)

- **React 18+**, TypeScript — обязательно (у нас 19).
- Единый репо, поднимается через **docker-compose**; фронт — `Dockerfile` (nginx, multi-stage).
- **README**, по которому проект запускается без устных пояснений (запуск, зависимости, сценарии, API, ограничения).
- **Задеплоено и доступно по ссылке** — жюри проверяет без локального запуска.
- Линтеры фронт+бэк с обоснованием; **юнит-тесты** на важных участках.
- **Использование AI указывать в README** (на каких этапах, как). AI в MVP = плюс к оценке.

## Что оценивает жюри (0–5 за пункт)

Бизнес-логика end-to-end · доп. функционал · README · тесты · линтер · дизайн и архитектура ·
продуктовые решения под бизнес-цель · чистота кода · доступность по ссылке.
**Два главных критерия — качество кода и бизнес-решение.**
