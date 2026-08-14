# swap-chain

`swap-chain` — монорепозиторий MVP-сервиса многостороннего обмена вещами.
Backend ищет замкнутые цепочки обмена, в которых каждый участник отдаёт свою вещь
и получает вещь, подходящую под его описание желаемого предмета.

## Что уже работает

- регистрация/вход по телефону с opaque cookie-сессией;
- создание карточек обмена с загрузкой фото и обязательным выбранным пользователем
  состоянием (`NEW`, `GOOD`, `USED`);
- подсказка описания, категории и состояния по фото через vision-модель; результат
  не сохраняется автоматически и подтверждается пользователем;
- классификация и векторизация через OpenRouter/Voyage с локальным Ollama fallback;
- обязательная ручная категория отдаваемой вещи и определение категории каждого
  текстового пожелания по цепочке Flash → embedding; неоднозначное пожелание
  переводит карточку в `ACTION_REQUIRED` до выбора владельца;
- matching engine: поиск замкнутых цепочек с дедупликацией по cycle key;
- создание, согласование и завершение обменных цепочек
  (`PENDING → ACCEPTED/REJECTED`, затем `ACCEPTED → COMPLETED`);
- личные диалоги соседей подобранной цепочки через long-polling;
- детерминированные anti-scam метки сообщений для ссылок, запросов кодов,
  паролей, платёжных данных и перехода во внешние мессенджеры;
- постоянный чат поддержки с подключением и отключением модераторов;
- жалобы на сообщение или пользователя, административная очередь решений и аудит;
- чёрный список, исключающий пару пользователей из matching и активных вариантов;
- отзывы после завершённого обмена и рассчитанная по ним репутация;
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

Первая загрузка скачает образы и настроенные в `.env` модели Ollama
(`bge-m3` и `llama3.1` в `.env.example`),
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

# Рабочий экран ПВЗ строится от цепочек, а передачи раскрываются внутри выбранной цепочки:
curl -b admin-cookies.txt \
  'http://localhost:8080/api/v1/admin/chains?status=ACCEPTED'

curl -b admin-cookies.txt \
  'http://localhost:8080/api/v1/admin/chains/12'

curl -X POST http://localhost:8080/api/v1/admin/deliveries/41/transition \
  -H "Content-Type: application/json" \
  -d '{"status":"AT_PVZ"}' \
  -b admin-cookies.txt

# После IN_DELIVERY сотрудник подтверждает выдачу входящей вещи конкретному участнику:
curl -X POST \
  http://localhost:8080/api/v1/admin/chains/12/participants/9/receipt \
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
`COMPLETED`. Та же транзакция переводит переданные вещи из `LOCKED` в
`EXCHANGED`, поэтому они больше не отображаются как участвующие в активной цепочке
и не могут вернуться в matching.

## Chat API

Пользователь видит отдельный диалог для каждой вещи, которую получает от соседа.
Собственные отдаваемые вещи без сообщений в список не попадают; после начала
разговора тред видят обе стороны, чтобы владелец мог ответить. Если одна и та же
вещь с тем же соседом встречается в нескольких цепочках, backend возвращает один
тред и одну общую историю. Диалоги доступны уже в статусе `PENDING`. Отправитель
берётся из opaque session cookie, а вещь и собеседник явно задаются в URL:

```bash
curl http://localhost:8080/api/v1/chat/threads \
  -b cookies.txt

curl -X POST http://localhost:8080/api/v1/items/44/chat/9/messages \
  -H "Content-Type: application/json" \
  -d '{"clientMessageId":"web-550e8400-e29b-41d4-a716-446655440000","text":"Встречаемся в ПВЗ?"}' \
  -b cookies.txt
```

Backend не блокирует доставку подозрительного сообщения, но добавляет в
`ChatMessage` поле `riskGroup`. Возможные значения: `CREDENTIALS`,
`VERIFICATION_CODE`, `PAYMENT_DETAILS`, `EXTERNAL_LINK` и
`OFF_PLATFORM_CONTACT`. Если известные признаки риска не найдены, поле отсутствует.
Классификация выполняется локально по стабильным правилам без ИИ и применяется
как к новым, так и к ранее сохранённым сообщениям.

```bash
curl -b cookies.txt \
  'http://localhost:8080/api/v1/items/44/chat/9/messages?afterId=0&limit=50&waitSeconds=25'

curl -X POST http://localhost:8080/api/v1/items/44/chat/9/read \
  -H "Content-Type: application/json" \
  -d '{"lastReadMessageId":81}' \
  -b cookies.txt
```

`clientMessageId` обязателен: повтор того же ID и текста возвращает исходное
сообщение с `200`, а новый ID создаёт сообщение с `201`. GET возвращает сообщения
по `id ASC`; если новых сообщений нет, он ждёт не более 25 секунд и отвечает
`200` с пустым `messages`. Список тредов содержит `item`, `counterpart`, последнее
сообщение, `hasUnread`, `unreadCount` и общий `totalUnreadCount`. У краткой вещи
есть `id`, `title` и первое `imageUrl`.
Отметка прочтения идемпотентна и не может сдвинуться назад. Для пользователя вне
цепочек с этой вещью ответ — `403`, для участника, который не является соседом
по её передаче, — `404`.

### Чат поддержки

Для каждого пользователя backend создаёт один постоянный support-тред с
приветственным системным сообщением. Он хранится на сервере и должен показываться
клиентом первым в общей вкладке сообщений. Пользователь получает тред через
`GET /api/v1/support/thread`, читает и отправляет сообщения через
`GET/POST /api/v1/support/messages`, а прочитанность сохраняет через
`POST /api/v1/support/read`. GET сообщений поддерживает тот же long-polling, что
и обычный чат.

Администратор видит очередь через `GET /api/v1/admin/support/threads`. Перед
отправкой сообщения он подключается к выбранному треду через
`POST /api/v1/admin/support/threads/{threadId}/join`; после работы отключается
через `POST .../leave`. Обе команды идемпотентны, а фактическое подключение или
отключение автоматически добавляет системное сообщение с именем модератора.
Писать в тред может только подключённый модератор. Изменения публикуются в
персональный SSE как `support.thread.updated`; событие служит сигналом повторно
загрузить тред и счётчик, а источником данных остаётся REST API.

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
| `PATCH /api/v1/items/{id}` | Изменить карточку или снять вещь с подбора |
| `GET /api/v1/items`, `GET /api/v1/items/{id}` | Список/одна карточка (только свои) |
| `POST /api/v1/vision/analyze` | Асинхронно предложить описание, категорию и состояние по фотографии |
| `GET /api/v1/items/{id}/matching` | Эфемерные цепочки для карточки (MATCHING-статус) |
| `POST /api/v1/chains` | Создать цепочку из matching-кандидатов |
| `GET /api/v1/chains`, `GET /api/v1/chains/{id}` | Список/детали цепочек |
| `POST /api/v1/chains/{id}/decision` | `APPROVED` или `DECLINED` |
| `POST /api/v1/chains/{id}/receipt` | Получатель подтверждает свою входящую вещь |
| `POST /api/v1/chains/{id}/reviews` | Оставить оценку непосредственному соседу после завершения обмена |
| `GET /api/v1/users/{id}/reviews` | Получить отзывы пользователя с cursor-пагинацией |
| `GET /api/v1/chat/threads` | Все личные диалоги и счётчик непрочитанных сообщений |
| `GET`, `POST /api/v1/items/{id}/chat/{counterpartId}/messages` | История/long-poll и отправка сообщений по передаваемой вещи |
| `POST /api/v1/items/{id}/chat/{counterpartId}/read` | Идемпотентная отметка сообщений прочитанными |
| `POST /api/v1/reports` | Пожаловаться на сообщение чата |
| `POST /api/v1/users/{id}/reports` | Пожаловаться на пользователя с опциональным контекстом цепочки |
| `GET`, `POST /api/v1/blocks` | Получить чёрный список или заблокировать пользователя |
| `DELETE /api/v1/blocks/{id}` | Разблокировать пользователя |
| `GET /api/v1/notifications`, `POST /api/v1/notifications/read` | Журнал уведомлений и отметка прочтения |
| `GET /api/v1/admin/deliveries` | Очередь товаров принятых цепочек для сотрудника ПВЗ |
| `POST /api/v1/admin/deliveries/{id}/transition` | Приём на ПВЗ, отправка или выдача получателю |
| `GET /api/v1/admin/metrics/funnel` | Снимок продуктовой воронки и причин распада цепочек (только ADMIN) |
| `GET /api/v1/admin/reports`, `GET /api/v1/admin/reports/{id}` | Очередь и детали жалоб (только ADMIN) |
| `POST /api/v1/admin/reports/{id}/assign` | Взять жалобу в работу |
| `POST /api/v1/admin/reports/{id}/decision` | Зафиксировать терминальное решение по жалобе |
| `GET /api/v1/admin/audit` | Неизменяемый журнал административных действий |
| `POST /api/v1/media` | Загрузить изображение (multipart) |
| `GET /api/v1/media/{objectKey}` | Получить изображение (публично) |
| `GET /api/v1/events` | Персональный SSE-поток |
| `GET /health` | Liveness HTTP-процесса без проверки внешних зависимостей |
| `GET /api/v1/health` | Готовность PostgreSQL, Ollama и справочника категорий |

Аутентификация — opaque HttpOnly cookie. Запросы из браузера с `credentials: "include"`.

## SSE события

- `item.status.updated` — карточка перешла `ANALYZING → MATCHING → LOCKED`;
- `vision.analysis.completed`, `vision.analysis.failed` — завершена подсказка по фото;
- `chain.created`, `chain.updated`, `chain.accepted`, `chain.rejected`;
- `notification.created` — журнал пользователя пополнился.

События приходят только участникам. После переподключения клиент восстанавливает
состояние через REST. События изменения доставки сначала записываются в
транзакционный outbox вместе с новым статусом, а затем публикуются фоновым worker.
Поэтому commit статуса не может произойти без постановки события; при аварии между
публикацией и подтверждением возможна повторная доставка с тем же SSE `id`.

## Конфигурация

| Переменная | Назначение | По умолчанию |
|---|---|---|
| `DATABASE_URL` | PostgreSQL URL | `postgres://swap_chain:...@127.0.0.1:5432/swap_chain?sslmode=disable` |
| `MIGRATIONS_URL` | Каталог миграций | `file://migrations` |
| `BACKEND_PORT` | Опубликованный порт API | `8080` |
| `HTTP_ADDR` | Адрес HTTP-сервера | `:8080` |
| `CORS_ALLOWED_ORIGIN` | Разрешённый origin | `http://localhost:18080` |
| `SESSION_TTL` | Время жизни сессии | `24h` |
| `COOKIE_SECURE` | Secure-флаг cookie | `false` |
| `OLLAMA_BASE_URL` | Адрес Ollama | `http://ollama:11434` |
| `OLLAMA_CHAT_MODEL` | Модель для анализа | `llama3.1` |
| `OLLAMA_EMBEDDINGS_MODEL` | Модель для эмбеддингов | `bge-m3` |
| `OLLAMA_TIMEOUT` | Таймаут одного запроса к локальной модели | `4m` |
| `OPENROUTER_API_KEY`, `OPENROUTER_MODEL` | Опциональный Flash-классификатор и vision/enrichment | пусто |
| `VOYAGE_API_KEY`, `VOYAGE_MODEL` | Опциональный embedding-провайдер; Ollama остаётся fallback | пусто / `voyageai/voyage-4-large` |
| `GIGACHAT_AUTH_KEY` | Опциональный Authorization Key дополнительного LLM | пусто |
| `MINIO_ACCESS_KEY` | Ключ MinIO | `minioadmin` |
| `MINIO_SECRET_KEY` | Секрет MinIO | `minioadmin` |
| `MINIO_BUCKET` | Бакет для медиа | `swap-chain-media` |
| `MINIO_PORT` | Порт MinIO API | `9000` |
| `MEDIA_MAX_UPLOAD_BYTES` | Максимальный размер фото | `10485760` (10 MiB) |
| `MATCHING_SIMILAR_ITEMS` | Кандидатов на узел | `20` |
| `MATCHING_CHAIN_LENGTH` | Макс. длина цепочки | `3` |
| `MATCHING_PENALTY_FACTOR` | Штраф за разброс score | `0.25` |
| `MATCHING_CHAIN_THRESHOLD` | Мин. итоговый score | `0.30` |
| `MATCHING_COMPATIBILITY_THRESHOLD` | Мин. совместимость конкретной пары вещей | `0.4` |
| `MATCHING_DEBUG` | Подробный лог кандидатов, рёбер и циклов matching | `false` |
| `ANALYSIS_BOOTSTRAP_TIMEOUT` | Таймаут проверки моделей и bootstrap категорий | `5m` |
| `ANALYSIS_TIMEOUT` | Общий таймаут полного анализа одной вещи | `5m` |
| `ANALYSIS_STALE_AFTER` | Возраст зависшего анализа до запуска recovery | `6m` |
| `SWAGGER_PORT` | Порт Swagger UI | `8081` |

Состояние влияет на matching через сохраняемый коэффициент качества: `NEW=1.0`,
`GOOD=0.8`, `USED=0.6`. Vision только предлагает значение; источником истины
остаётся выбор пользователя из `POST/PATCH /items`.

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
Миграция `000026` переводит идентичность треда на
`(item, user, counterpart)`, объединяет одну вещь из нескольких цепочек и задаёт
уникальный ключ `(item, sender, counterpart, clientMessageId)` для идемпотентной
отправки.

Миграция `000008` добавляет конечные статусы доставки `RECEIVED` и цепочки
`COMPLETED`. Подтверждение получателя, аудит и возможное завершение всей цепочки
записываются атомарно.

Миграция `000010` выравнивает схему анализа и matching: оставляет единые локальные
embedding-поля, делает `image_amount` вычисляемым и добавляет системный справочник
категорий для bootstrap при старте.

Миграция `000011` добавляет надёжную очередь `matching_jobs`. Завершение анализа
атомарно переводит вещь в `MATCHING` и ставит задание, а фоновый worker находит
циклы и сохраняет их как `PENDING`-цепочки. Миграция также ставит задания для
всех вещей, которые уже находились в `MATCHING`; ошибки повторяются с backoff,
зависшие leases подхватываются снова после перезапуска процесса.

Миграция `000015` добавляет неизменяемые отзывы и агрегат репутации. Оценивать
можно только непосредственного соседа в завершённой цепочке и только один раз;
профиль показывает nullable-рейтинг, число отзывов и завершённых обменов.
Актуальный рейтинг используется matching вместо исходного демонстрационного
значения пользователя.

Миграция `000016` добавляет транзакционный outbox для изменений доставки.
Конкурирующие backend-инстансы забирают записи через lease и
`FOR UPDATE SKIP LOCKED`, а PostgreSQL `LISTEN/NOTIFY` разносит событие по локальным
SSE hub каждого инстанса; после рестарта неподтверждённые записи обрабатываются
повторно. SSE остаётся сигналом обновить данные, а источником актуального состояния
является REST API.
Причины и гарантии решения зафиксированы в
[ADR 0001](docs/decisions/0001-transactional-outbox-for-realtime.md).

Миграция `000017` добавляет причины распада цепочек. Причина записывается
атомарно с переходом цепочки в `REJECTED`; старые отклонённые цепочки получают
значение `unknown`. Ответы `GET /api/v1/chains` и `GET /api/v1/chains/{id}` для
каждого участника содержат статус и время последнего изменения его входящей
доставки: `incomingDeliveryStatus` и `incomingDeliveryUpdatedAt`.

Миграция `000024` добавляет двусторонний чёрный список и модерацию жалоб на
сообщения. Миграция `000029` расширяет ту же очередь жалобами на пользователей,
которые могут содержать контекст цепочки без обязательной ссылки на сообщение.
Назначение, терминальное решение (`resolved`/`rejected`) и actor фиксируются в
административном аудите. Блокировка пользователя остаётся отдельным действием.

Миграция `000030` делает состояние карточки пользовательским сохраняемым полем:
старые данные переводятся в один из трёх коэффициентов, новые запросы обязаны
передавать `NEW`, `GOOD` или `USED`.

`GET /api/v1/admin/metrics/funnel` возвращает согласованный снимок из PostgreSQL:

- время до первого варианта — среднее число секунд между созданием вещи и первой
  сохранённой цепочкой;
- доля вещей с цепочкой — доля текущих `MATCHING`/`LOCKED` вещей, участвовавших
  хотя бы в одной сохранённой цепочке;
- acceptance rate — доля `ACCEPTED` и `COMPLETED` среди цепочек, по которым уже
  принято решение (`ACCEPTED`, `COMPLETED`, `REJECTED`);
- завершение доставки — доля `COMPLETED` среди цепочек, достигших `ACCEPTED`;
- причины распада — количество отклонённых цепочек по фиксированным причинам.

Если у коэффициента нет знаменателя, API возвращает `null`, а не вводящее в
заблуждение значение `0`.

Backend не стартует без PostgreSQL, обеих моделей Ollama и заполненных embeddings
категорий. `GET /health` проверяет только liveness процесса. Версионированный
`GET /api/v1/health` проверяет PostgreSQL, модели Ollama и справочник категорий,
возвращая 503, пока сервис не готов принимать пользовательские запросы.

## Линтеры, тесты и сборка

Backend проверяется из корня репозитория:

```bash
make lint-go
make test-go
TEST_DATABASE_URL=postgres://swap_chain:swap_chain@127.0.0.1:5432/swap_chain?sslmode=disable make test-integration
make build
```

Обычный `go test ./...` пропускает PostgreSQL integration-тесты, если
`TEST_DATABASE_URL` не задан. `make test-integration` создаёт для прогона
изолированные базы, но указанный сервер PostgreSQL должен разрешать создание и
удаление баз. Не направляйте эту команду на общую или production-базу.

Frontend проверяется отдельно:

```bash
pnpm --dir frontend run lint
VITE_API_URL= pnpm --dir frontend run test
pnpm --dir frontend run build
```

Эквивалент подготовки тестового режима в PowerShell:

```powershell
Remove-Item Env:VITE_API_URL -ErrorAction SilentlyContinue
pnpm --dir frontend run test
```

Пустой `VITE_API_URL` включает предусмотренный unit-тестами детерминированный
mock-режим. Production-сборка Compose передаёт `VITE_API_URL=/` и работает с API
через nginx того же origin.

### Почему включены эти правила линтера

Backend использует `golangci-lint`; точный исполняемый набор является частью
репозитория и находится в `.golangci.yaml`:

| Проверка | Зачем она нужна |
|---|---|
| `bodyclose` | Находит незакрытые HTTP response bodies, которые мешают повторному использованию соединений и могут исчерпать ресурсы. |
| `errcheck` | Не позволяет молча терять ошибки записи, закрытия ресурсов и других операций с побочными эффектами. |
| `errorlint` | Сохраняет корректную работу `errors.Is`/`errors.As` и цепочек ошибок через `%w`. |
| `gocritic` | Находит подозрительные и избыточные конструкции, которые легко расходятся по поведению при последующих изменениях. |
| `govet` | Выполняет стандартные проверки Go с учётом типов, форматных строк и конкурентного доступа. |
| `ineffassign` | Удаляет присваивания, результат которых никогда не используется и часто скрывает ошибку в ветвлении. |
| `misspell` | Защищает имена, сообщения об ошибках и документацию от повторяющихся опечаток. |
| `revive` | Проверяет базовую сопровождаемость Go-кода и опасные соглашения об именовании. |
| `staticcheck` | Находит дефекты API, конкурентности и стандартной библиотеки, которые компилятор обычно допускает. |
| `unused` | Не даёт оставлять мёртвые функции, константы и зависимости. |
| `gofmt`, `goimports` | Обеспечивают единый формат и детерминированную организацию импортов. |

Для `revive` исключены только сообщения `exported` и `package-comments`.
Большая часть Go-пакетов является внутренней реализацией монорепозитория, а
публичный HTTP-контракт документируется в `api/openapi.yaml`. Обязательные GoDoc-
комментарии на каждую технически экспортируемую сущность создавали бы большой
объём формального текста без дополнительной защиты корректности. Остальные
проверки `revive` продолжают выполняться.

Generated-файлы `internal/api/*.gen.go` и `shared/db/*.sql.go` не исправляются
вручную: они воспроизводимо создаются из OpenAPI и SQL через `make generate`.
Для них отключены только проверки, которые должен исправлять генератор; обычный
код остаётся под полным набором правил.

Frontend использует `oxlint`: он быстро проверяет JavaScript/TypeScript и React,
включая правило совместимости компонентов с Fast Refresh. TypeScript-компилятор
в `pnpm build` дополнительно отвечает за типы, поэтому lint, tests и build не
заменяют друг друга и запускаются все три.

## Известные ограничения

- Ollama в Docker работает CPU-only; для GPU-ускорения установите Ollama на хосте и задайте `OLLAMA_BASE_URL=http://host.docker.internal:11434` в `.env`;
- CI и production deployment пока не добавлены;
- vision зависит от доступности настроенного внешнего провайдера или локальной модели;
- привязка администраторов и доставок к нескольким конкретным ПВЗ пока не реализована.

Общие правила архитектуры, миграций, тестирования и работы с ветками описаны в
[AGENTS.md](AGENTS.md).
