# swap-chain

`swap-chain` — монорепозиторий MVP-сервиса многостороннего обмена вещами.
Backend ищет замкнутые цепочки обмена, в которых каждый участник отдаёт свою вещь
и получает вещь, подходящую под его описание желаемого предмета.

## Что уже работает

- регистрация/вход по телефону с opaque cookie-сессией;
- создание карточек обмена с загрузкой фото;
- автоматическое распознавание и векторизация через Ollama (bge-m3 + chat model);
- matching engine: поиск замкнутых цепочек с дедупликацией по cycle key;
- создание и согласование обменных цепочек (PENDING → ACCEPTED/REJECTED);
- персональный SSE для real-time уведомлений;
- хранение и выдача изображений через MinIO (только через backend);
- PostgreSQL 17 с pgvector, версионируемые up/down миграции;
- Swagger UI с актуальным OpenAPI-контрактом;
- отдельный demo seed для воспроизводимых демонстраций;
- Docker Compose для полного локального запуска.

## Структура

```text
swap-chain/
├── api/                 # OpenAPI-контракт
├── backend/             # Go backend, matching, analyze и миграции
├── frontend/            # React/TypeScript frontend
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
- GNU Make для коротких команд (в Windows — установленный через Chocolatey `make`);
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

Затем из корня репозитория:

```bash
make config
make up
```

Или явно через Docker Compose:

```bash
docker compose up -d --build
```

Первая загрузка скачает образы и модели Ollama (`bge-m3` и `llama3.1`),
что может занять 10–15 минут. Следить за процессом можно командой:

```bash
docker compose logs -f ollama-pull
```

`make up` собирает backend, поднимает PostgreSQL+pgvector, Ollama, MinIO,
загружает модели, применяет миграции и только после bootstrap embeddings категорий
запускает готовый API. Frontend ждёт успешный readiness backend.
После старта доступны:

- frontend: <http://localhost:18080>;
- backend: <http://localhost:8080>;
- liveness: <http://localhost:8080/health>;
- readiness: <http://localhost:8080/api/v1/health>;
- Swagger UI: <http://localhost:8081>;
- MinIO console: <http://localhost:9001> (если опубликован порт).

Остановить сервисы без удаления данных:

```bash
make down
```

Тома PostgreSQL и MinIO сохраняются. Разрушительная команда `docker compose down -v`
намеренно не завернута в Makefile.

## Demo seed

Для создания воспроизводимых тестовых данных (3 пользователя: Алиса/Борис/Вера
и 3 карточки с гарантированным 3-циклом):

```bash
cd backend && DATABASE_URL=postgres://swap_chain:swap_chain@127.0.0.1:5432/swap_chain?sslmode=disable go run ./cmd/demo-seed
```

После seed можно проверить matching для Алисы (телефон `+79001000001`):

```bash
curl -X POST http://localhost:8080/api/v1/session \
  -H "Content-Type: application/json" \
  -d '{"phone":"+79001000001"}' -c cookies.txt

curl http://localhost:8080/api/v1/items/55/matching -b cookies.txt
```

## Локальный запуск Go backend

Можно оставить в Docker только PostgreSQL и MinIO, а Go-процессы запускать на хосте:

```bash
docker compose up -d postgres minio
make migrate-up
make run-backend
```

Команды миграций используют `DATABASE_URL`, файлы берут из `MIGRATIONS_URL`.

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
make test-integration запустить миграционные и сквозные PostgreSQL-тесты
```

Все команды Makefile имеют прямой эквивалент, например `cd backend && go test
./...` или `docker compose up -d --build`.

## API

Источник контракта — [api/openapi.yaml](api/openapi.yaml). Ключевые endpoint:

| Маршрут | Назначение |
|---|---|
| `POST /api/v1/users` | Регистрация (имя + телефон), возвращает session cookie |
| `POST /api/v1/session` | Вход по телефону |
| `GET /api/v1/session` | Текущая сессия (user id, username, phone) |
| `DELETE /api/v1/session` | Выход |
| `POST /api/v1/items` | Создать карточку обмена |
| `GET /api/v1/items`, `GET /api/v1/items/{id}` | Список/одна карточка (только свои) |
| `GET /api/v1/items/{id}/matching` | Эфемерные цепочки для карточки (MATCHING-статус) |
| `POST /api/v1/chains` | Создать цепочку из matching-кандидатов |
| `GET /api/v1/chains`, `GET /api/v1/chains/{id}` | Список/детали цепочек |
| `POST /api/v1/chains/{id}/decision` | `APPROVED` или `DECLINED` |
| `POST /api/v1/media` | Загрузить изображение (multipart) |
| `GET /api/v1/media/{objectKey}` | Получить изображение (публично) |
| `GET /api/v1/events` | Персональный SSE-поток |
| `GET /api/v1/health` | Готовность backend и PostgreSQL |

Аутентификация — opaque HttpOnly cookie. Запросы из браузера с `credentials: "include"`.

## SSE события

- `item.status.updated` — карточка перешла ANALYZING → MATCHING → LOCKED
- `chain.created`, `chain.updated`, `chain.accepted`, `chain.rejected`

События приходят только участникам. После переподключения клиент восстанавливает
состояние через REST.

## Конфигурация

| Переменная | Назначение | По умолчанию |
|---|---|---|
| `DATABASE_URL` | PostgreSQL URL | `postgres://swap_chain:...@127.0.0.1:5432/swap_chain?sslmode=disable` |
| `MIGRATIONS_URL` | Каталог миграций | `file://migrations` |
| `BACKEND_PORT` | Опубликованный порт API | `8080` |
| `HTTP_ADDR` | Адрес HTTP-сервера | `:8080` |
| `CORS_ALLOWED_ORIGIN` | Разрешённый origin | `http://localhost:5173` |
| `SESSION_TTL` | Время жизни сессии | `24h` |
| `COOKIE_SECURE` | Secure-флаг cookie | `false` |
| `OLLAMA_BASE_URL` | Адрес Ollama | `http://localhost:11434` |
| `OLLAMA_CHAT_MODEL` | Модель для анализа | `llama3.1` |
| `OLLAMA_EMBEDDINGS_MODEL` | Модель для эмбеддингов | `bge-m3` |
| `MINIO_ACCESS_KEY` | Ключ MinIO | `minioadmin` |
| `MINIO_SECRET_KEY` | Секрет MinIO | `minioadmin` |
| `MINIO_BUCKET` | Бакет для медиа | `swap-chain-media` |
| `MINIO_PORT` | Порт MinIO API | `9000` |
| `MEDIA_MAX_UPLOAD_BYTES` | Максимальный размер фото | `10485760` (10 MiB) |
| `MATCHING_SIMILAR_ITEMS` | Кандидатов на узел | `20` |
| `MATCHING_CHAIN_LENGTH` | Макс. длина цепочки | `3` |
| `MATCHING_PENALTY_FACTOR` | Штраф за разброс score | `0.25` |
| `MATCHING_CHAIN_THRESHOLD` | Мин. итоговый score | `0.30` |
| `ANALYSIS_BOOTSTRAP_TIMEOUT` | Таймаут проверки моделей и bootstrap категорий | `5m` |
| `SWAGGER_PORT` | Порт Swagger UI | `8081` |

`.env` не коммитится. Значения примера — только для локальной разработки.

## База данных и миграции

Миграции включают pgvector, создают `users`, `items`, `chains` и `chain_participants`,
а также версионированный справочник категорий. Embeddings системных категорий
вычисляются настроенной Ollama-моделью при старте backend и повторно не создаются.
Внешние ключи и уникальные ограничения защищают связь вещи с владельцем, запрещают
повтор вещи или пользователя внутри цепочки, гарантируют уникальность cycle key.
Embedding — `vector(1024)`, HNSW-индекс для cosine distance.

Backend не стартует без PostgreSQL, обеих моделей Ollama и заполненных embeddings
категорий. `GET /health` проверяет только liveness процесса. Версионированный
`GET /api/v1/health` проверяет PostgreSQL, модели Ollama и справочник категорий,
возвращая 503, пока сервис не готов принимать пользовательские запросы.

## Линтер и тесты

```bash
make test-go
TEST_DATABASE_URL=postgres://swap_chain:swap_chain@127.0.0.1:5432/swap_chain?sslmode=disable make test-integration
make lint-go
make build
```

## Известные ограничения

- Ollama в Docker работает CPU-only; для GPU-ускорения установите Ollama на хосте и задайте `OLLAMA_BASE_URL=http://host.docker.internal:11434` в `.env`;
- CI и production deployment пока не добавлены;
- фото-распознавание (GigaChat/CV) не подключено к API, только analyze-пайплайн;
- рейтинг, доставка, подтверждение физического обмена — post-MVP.

Общие правила архитектуры, миграций, тестирования и работы с ветками описаны в
[AGENTS.md](AGENTS.md).
