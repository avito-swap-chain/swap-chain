# swap-chain

`swap-chain` — монорепозиторий MVP-сервиса многостороннего обмена вещами.
Backend ищет замкнутые цепочки обмена, в которых каждый участник отдаёт свою вещь
и получает вещь, подходящую под его описание желаемого предмета.

## Что уже работает

- Go matching engine и unit-тесты;
- PostgreSQL 17 с расширением pgvector;
- версионируемые up/down-миграции через `golang-migrate`;
- HTTP backend с проверкой БД и запуском matching по идентификатору вещи;
- Swagger UI с актуальным OpenAPI-контрактом;
- Docker Compose для полного локального запуска.

Frontend в эту ветку намеренно не включён и не изменяется.

## Структура

```text
swap-chain/
├── api/                 # OpenAPI-контракт
├── backend/             # Go backend, matching и миграции
├── frontend/            # зона React/TypeScript frontend
├── .env.example         # пример локальной конфигурации без секретов
├── .golangci.yaml       # правила анализа Go
├── AGENTS.md            # общие правила работы с проектом
├── docker-compose.yml   # PostgreSQL, миграции, backend и Swagger UI
├── Makefile             # команды разработки
└── README.md
```

## Требования

- Git;
- Docker с поддержкой `docker compose`;
- GNU Make для коротких команд (в Windows можно использовать установленный
  через Chocolatey `make`);
- Go 1.26.5 и golangci-lint v2.12.2 для запуска backend вне контейнера.

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

`make up` собирает backend, поднимает pgvector, дожидается готовности БД,
применяет миграции и только после этого запускает API. После запуска доступны:

- backend: <http://localhost:8080>;
- health check: <http://localhost:8080/health>;
- Swagger UI: <http://localhost:8081>;
- PostgreSQL: `localhost:5432`.

Проверка matching для существующей вещи:

```bash
curl http://localhost:8080/items/1/matching
```

Остановить сервисы без удаления данных:

```bash
make down
```

Том PostgreSQL сохраняется. Разрушительная команда `docker compose down -v`
намеренно не завернута в Makefile.

## Локальный запуск Go backend

Можно оставить в Docker только БД, а Go-процессы запускать на хосте:

```bash
docker compose up -d postgres
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
```

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
| `MATCHING_SIMILAR_ITEMS` | кандидатов из БД на узел | `20` |
| `MATCHING_CHAIN_LENGTH` | максимальная длина цепочки | `4` |
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
- `GET /items/{itemId}/matching` — поиск циклов для вещи.

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
- CI и production deployment пока не добавляются.

Общие правила архитектуры, миграций, тестирования и работы с ветками описаны в
[AGENTS.md](AGENTS.md).
