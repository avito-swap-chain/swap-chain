# swap-chain

`swap-chain` — монорепозиторий MVP-сервиса многостороннего обмена вещами.
Backend ищет замкнутые цепочки обмена, в которых каждый участник отдаёт свою вещь
и получает вещь, подходящую под его описание желаемого предмета.

## Что уже работает

- регистрация/вход по телефону с opaque cookie-сессией;
- создание карточек обмена с загрузкой фото;
- автоматическое распознавание и векторизация через Ollama (bge-m3 + chat model);
- matching engine: поиск замкнутых цепочек с дедупликацией по cycle key;
- создание, согласование и завершение обменных цепочек
  (`PENDING → ACCEPTED/REJECTED`, затем `ACCEPTED → COMPLETED`);
- личные диалоги соседей подобранной цепочки через long-polling;
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
├── backend/             # Go backend, migrations and modules/{analyze,matching,chat,admin}
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

Миграция `000006` всегда создаёт зарезервированного сотрудника ПВЗ для локального
MVP-демо:

- имя: `ПВЗ Администратор (demo)`;
- телефон для входа без пароля: `+79009999999`;
- роль: `ADMIN`.

Для запуска и проверки admin API из корня проекта:

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps -a

curl http://localhost:8080/api/v1/health

curl -X POST http://localhost:8080/api/v1/session \
  -H "Content-Type: application/json" \
  -d '{"phone":"+79009999999"}' \
  -c admin-cookies.txt

curl -b admin-cookies.txt \
  'http://localhost:8080/api/v1/admin/deliveries?status=AWAITING_PVZ'

curl -X POST http://localhost:8080/api/v1/admin/deliveries/41/transition \
  -H "Content-Type: application/json" \
  -d '{"status":"AT_PVZ"}' \
  -b admin-cookies.txt

curl -X POST http://localhost:8080/api/v1/admin/deliveries/41/transition \
  -H "Content-Type: application/json" \
  -d '{"status":"IN_DELIVERY"}' \
  -b admin-cookies.txt

# Выдачу конкретной вещи может подтвердить сотрудник ПВЗ:
curl -X POST http://localhost:8080/api/v1/admin/deliveries/41/transition \
  -H "Content-Type: application/json" \
  -d '{"status":"RECEIVED"}' \
  -b admin-cookies.txt
```

Во всех командах перехода `41` нужно заменить на `id` из ответа списка.
Передачи появляются только после того, как все участники приняли цепочку и она
перешла в `ACCEPTED`.

Получатель может подтвердить собственную входящую вещь без передачи её ID —
backend определяет её по участнику из session cookie и цепочке:

```bash
curl -X POST http://localhost:8080/api/v1/chains/12/receipt \
  -b recipient-cookies.txt
```

Подтверждение доступно только из `IN_DELIVERY` и идемпотентно. Когда все две
или три вещи получили статус `RECEIVED`, та же транзакция переводит цепочку в
`COMPLETED`. Вещи намеренно остаются `LOCKED`: отдельного статуса `EXCHANGED`
пока нет, а возврат в `MATCHING` создал бы повторные обмены уже переданных вещей.

## Chat API

Для каждой подобранной цепочки пользователь видит отдельные диалоги только с
соседями по обменному кольцу: с тем, чью вещь он получает, и с тем, кто получает
его вещь. Диалоги доступны уже в статусе `PENDING`. Отправитель берётся из opaque
session cookie, а собеседник явно задаётся в URL:

```bash
curl http://localhost:8080/api/v1/chat/threads \
  -b cookies.txt

curl -X POST http://localhost:8080/api/v1/chains/12/chat/9/messages \
  -H "Content-Type: application/json" \
  -d '{"clientMessageId":"web-550e8400-e29b-41d4-a716-446655440000","text":"Встречаемся в ПВЗ?"}' \
  -b cookies.txt

curl -b cookies.txt \
  'http://localhost:8080/api/v1/chains/12/chat/9/messages?afterId=0&limit=50&waitSeconds=25'

curl -X POST http://localhost:8080/api/v1/chains/12/chat/9/read \
  -H "Content-Type: application/json" \
  -d '{"lastReadMessageId":81}' \
  -b cookies.txt
```

`clientMessageId` обязателен: повтор того же ID и текста возвращает исходное
сообщение с `200`, а новый ID создаёт сообщение с `201`. GET возвращает сообщения
по `id ASC`; если новых сообщений нет, он ждёт не более 25 секунд и отвечает
`200` с пустым `messages`. Список тредов содержит `counterpart`, `giveItem`,
`receiveItem`, последнее сообщение, `hasUnread`, `unreadCount` и общий
`totalUnreadCount`. У краткой вещи есть `id`, `title` и первое `imageUrl`; для
трёхсторонней цепочки одно из направлений конкретного диалога может быть `null`.
Отметка прочтения идемпотентна и не может сдвинуться назад. Для пользователя вне
цепочки ответ — `403`, для участника, который не является соседом в этой
цепочке, — `404`.

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
| `POST /api/v1/chains/{id}/receipt` | Получатель подтверждает свою входящую вещь |
| `GET /api/v1/chat/threads` | Все личные диалоги и счётчик непрочитанных сообщений |
| `GET`, `POST /api/v1/chains/{id}/chat/{counterpartId}/messages` | История/long-poll и отправка личных сообщений |
| `POST /api/v1/chains/{id}/chat/{counterpartId}/read` | Идемпотентная отметка сообщений прочитанными |
| `GET /api/v1/admin/deliveries` | Очередь товаров принятых цепочек для сотрудника ПВЗ |
| `POST /api/v1/admin/deliveries/{id}/transition` | Приём на ПВЗ, отправка или выдача получателю |
| `POST /api/v1/media` | Загрузить изображение (multipart) |
| `GET /api/v1/media/{objectKey}` | Получить изображение (публично) |
| `GET /api/v1/events` | Персональный SSE-поток |
| `GET /health` | Liveness HTTP-процесса без проверки внешних зависимостей |
| `GET /api/v1/health` | Готовность PostgreSQL, Ollama и справочника категорий |

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
| `OLLAMA_TIMEOUT` | Таймаут одного запроса к локальной модели | `4m` |
| `GIGACHAT_AUTH_KEY` | Опциональный Authorization Key основного LLM; при ошибке используется Ollama | пусто |
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
| `ANALYSIS_TIMEOUT` | Общий таймаут полного анализа одной вещи | `5m` |
| `ANALYSIS_STALE_AFTER` | Возраст зависшего анализа до запуска recovery | `6m` |
| `SWAGGER_PORT` | Порт Swagger UI | `8081` |

`.env` не коммитится. Значения примера — только для локальной разработки.

## База данных и миграции

Миграции включают pgvector, создают `users`, `items`, `chains` и `chain_participants`,
а также версионированный справочник категорий. Embeddings системных категорий
вычисляются настроенной Ollama-моделью при старте backend и повторно не создаются.
Внешние ключи и уникальные ограничения защищают связь вещи с владельцем, запрещают
повтор вещи или пользователя внутри цепочки, гарантируют уникальность cycle key.
Embedding — `vector(1024)`, HNSW-индекс для cosine distance.

Миграция `000006` добавляет роль `ADMIN`, статусы физической передачи товара и
неизменяемый аудит действий сотрудника ПВЗ. Повтор одной и той же команды
идемпотентен; приём и отправка проверяются и записываются в одной транзакции.

Миграция `000007` добавляет личные сообщения, read-watermark и индексы тредов.
Уникальный ключ `(chain, sender, counterpart, clientMessageId)` обеспечивает
идемпотентную отправку внутри конкретного диалога.

Миграция `000008` добавляет конечные статусы доставки `RECEIVED` и цепочки
`COMPLETED`. Подтверждение получателя, аудит и возможное завершение всей цепочки
записываются атомарно.

Миграция `000010` выравнивает схему анализа и matching: оставляет единые локальные
embedding-поля, делает `image_amount` вычисляемым и добавляет системный справочник
категорий для bootstrap при старте.

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
- привязка администраторов и доставок к нескольким конкретным ПВЗ пока не реализована.

Общие правила архитектуры, миграций, тестирования и работы с ветками описаны в
[AGENTS.md](AGENTS.md).
