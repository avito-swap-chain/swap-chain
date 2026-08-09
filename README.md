# swap-chain

`swap-chain` — монорепозиторий MVP-сервиса многостороннего обмена вещами.
Backend ищет замкнутые цепочки обмена, в которых каждый участник отдаёт свою вещь
и получает вещь, подходящую под его описание желаемого предмета.

## Что уже работает

- Go matching engine и unit-тесты;
- PostgreSQL 17 с расширением pgvector;
- версионируемые up/down-миграции через `golang-migrate`;
- HTTP backend с проверкой БД и запуском matching по идентификатору вещи;
- единый OpenAPI strict transport, валидация запросов и JSON-ошибки;
- demo-session через непрозрачный HttpOnly cookie и персональные SSE-события;
- приватное MinIO-хранилище с загрузкой и выдачей изображений через backend;
- React/Vite frontend, generated TypeScript API-типы и клиент с cookie credentials;
- Swagger UI с актуальным OpenAPI-контрактом;
- Docker Compose для полного локального запуска.

React-приложение в `frontend/` пока использует mock API. Подключение его экранов
к backend и персональному SSE выполняется отдельно от merge.

## Структура

```text
swap-chain/
├── api/                 # OpenAPI-контракт
├── backend/             # Go backend, matching и миграции
├── frontend/            # зона React/TypeScript frontend
├── .env.example         # пример локальной конфигурации без секретов
├── .golangci.yaml       # правила анализа Go
├── AGENTS.md            # общие правила работы с проектом
├── docker-compose.yml   # PostgreSQL, MinIO, миграции, backend и Swagger UI
├── Makefile             # команды разработки
└── README.md
```

## Требования

- Git;
- Docker с поддержкой `docker compose`;
- GNU Make для коротких команд (в Windows можно использовать установленный
  через Chocolatey `make`);
- Go 1.26.5 и golangci-lint v2.12.2 для запуска backend вне контейнера.
- Node.js 24+ и pnpm 11.3.0 для frontend.

Если GoLand оставил ошибочный `GOROOT=D:\Goland` в текущем PowerShell-сеансе,
удалите только эту переменную процесса перед запуском Go:

```powershell
Remove-Item Env:GOROOT -ErrorAction SilentlyContinue
go version
```

## Быстрый старт через Docker Compose

Создайте локальный файл окружения:

```powershell
Copy-Item .env.example .env
```

Затем из корня репозитория выполните:

```bash
make config
make up
```

`make up` собирает backend, поднимает pgvector и приватный MinIO, применяет
миграции и только после этого запускает API. После запуска доступны:

- backend: <http://localhost:8080>;
- health check: <http://localhost:8080/health>;
- Swagger UI: <http://localhost:8081>;
- PostgreSQL: `localhost:5432`.

Проверка matching для существующей вещи:

```bash
curl --cookie cookies.txt http://localhost:8080/api/v1/items/1/matching
```

Остановить сервисы без удаления данных:

```bash
make down
```

Тома PostgreSQL и MinIO сохраняются. Разрушительная команда `docker compose down -v`
намеренно не завернута в Makefile.

## Локальный запуск Go backend

Можно оставить в Docker только БД, а Go-процессы запускать на хосте:

```bash
docker compose up -d postgres minio
make migrate-up
make run-backend
```

Команды миграций используют `DATABASE_URL`, а файлы берут из `MIGRATIONS_URL`.
При запуске из `backend/` безопасные локальные значения по умолчанию совпадают с
`.env.example`.

## Основные команды

```text
make help             показать все команды
make config           проверить конфигурацию Compose
make generate         пересобрать Go/TypeScript код из OpenAPI
make build            собрать все Go-команды
make run-backend      запустить API на хосте
make up               поднять полное локальное окружение
make down             остановить окружение без удаления данных
make logs             читать логи сервисов
make migrate-up       применить все новые миграции
make migrate-down     откатить одну миграцию
make migrate-version  показать версию схемы
make lint             запустить golangci-lint
make test             запустить доступные тесты
make test-go          запустить Go-тесты
make typecheck-frontend проверить TypeScript API-клиент
```

Для frontend отдельно доступны `pnpm --dir frontend dev`, `pnpm --dir frontend test`
и `pnpm --dir frontend build`.

Все команды Makefile имеют прямой эквивалент, например `cd backend && go test
./...` или `docker compose up -d --build`.

## Конфигурация

| Переменная | Назначение | Значение в примере |
|---|---|---|
| `DATABASE_URL` | PostgreSQL URL для локального Go-процесса | `postgres://swap_chain:...@127.0.0.1:5432/swap_chain?sslmode=disable` |
| `MIGRATIONS_URL` | каталог миграций | `file://migrations` |
| `BACKEND_PORT` | опубликованный Compose-порт API | `8080` |
| `HTTP_ADDR` | адрес локального Go HTTP-сервера | `:8080` |
| `DB_CONNECT_TIMEOUT` | таймаут проверки БД при старте | `10s` |
| `SHUTDOWN_TIMEOUT` | таймаут корректной остановки HTTP | `10s` |
| `CORS_ALLOWED_ORIGIN` | frontend origin, которому разрешены cookie-запросы | `http://localhost:5173` |
| `SESSION_TTL` | срок жизни demo-session | `24h` |
| `COOKIE_SECURE` | отправлять session cookie только по HTTPS | `false` |
| `MINIO_ENDPOINT` | endpoint MinIO для локального Go-процесса | `localhost:9000` |
| `MINIO_PORT` | loopback-only порт MinIO для локального Go-процесса | `9000` |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | локальные credentials MinIO | `minioadmin` |
| `MINIO_BUCKET` | приватный bucket изображений | `swap-chain-media` |
| `MINIO_USE_SSL` | использовать TLS между backend и MinIO | `false` |
| `MEDIA_MAX_UPLOAD_BYTES` | максимальный размер одного изображения | `10485760` |
| `MATCHING_SIMILAR_ITEMS` | кандидатов из БД на узел | `20` |
| `MATCHING_CHAIN_LENGTH` | максимальная длина цепочки | `3` |
| `MATCHING_PENALTY_FACTOR` | штраф за разброс score | `0.25` |
| `MATCHING_CHAIN_THRESHOLD` | минимальный итоговый score | `0.30` |
| `POSTGRES_*` | локальные имя БД, пользователь, пароль и порт | `swap_chain`, `5432` |
| `SWAGGER_PORT` | опубликованный порт Swagger UI | `8081` |

`.env` не коммитится. Значения примера предназначены только для локальной
разработки.

## База данных и миграции

Первая миграция включает pgvector и создаёт `users`, `items`, `chains` и
`chain_items`. Внешние ключи и уникальные ограничения защищают связь вещи с её
владельцем и запрещают повтор вещи или пользователя внутри одной цепочки.
Embedding хранится как `vector(1024)`; для поиска по cosine distance создаётся
HNSW-индекс.

Backend не стартует, если не может подключиться к PostgreSQL. `GET /health`
также выполняет реальный `PingContext` и возвращает `503`, если БД недоступна.

## API

Источником истины является [api/openapi.yaml](api/openapi.yaml). Сейчас
реализованы:

- `GET /health` — готовность backend и PostgreSQL;
- `GET /api/v1/health` — версия health endpoint в общем API;
- `POST /api/v1/users`, `POST|GET|DELETE /api/v1/session` — регистрация, вход по телефону, current session и выход;
- `GET|POST /api/v1/items`, `GET /api/v1/items/{itemId}` — карточки обмена;
- `POST /api/v1/media`, `GET /api/v1/media/{objectKey}` — загрузка изображения и постоянная выдача через backend;
- `GET /api/v1/items/{itemId}/matching` — поиск эфемерных циклов для своей вещи;
- chain endpoints — сохранение выбранной цепочки, чтение и решения участников;
- `GET /api/v1/events` — персональный SSE stream текущей demo-session;

Demo-session нужна только для хакатонного сценария без полноценного входа. Она
изолирует запросы и события пользователей, но вход только по номеру телефона не
подтверждает владение номером. После перезапуска backend сессии исчезают.

## Линтер и тесты

`.golangci.yaml` фиксирует единый набор проверок: `govet`, `staticcheck`,
`errcheck`, `bodyclose`, `errorlint`, `gocritic`, `revive`, форматирование и
другие базовые правила. Такой набор ловит потерянные ошибки и подозрительные
конструкции, особенно опасные рядом с БД и конкурентной бизнес-логикой.

```bash
make test-go
make lint-go
make build
```

## Известные ограничения

- текущий matching использует одну пару `want_embedding`/`offer_embedding`, а не
  полноценный список пожеланий;
- алгоритмические дефекты циклов и резервирование вещей исправляются отдельно от
  инфраструктурной интеграции;
- Ollama нужен только для сценария создания embeddings и пока не включён в
  Compose;
- распознавание содержимого загруженных фотографий пока не подключено;
- demo-session не является production-аутентификацией;
- CI и production deployment пока не добавляются.

Общие правила архитектуры, миграций, тестирования и работы с ветками описаны в
[AGENTS.md](AGENTS.md).
