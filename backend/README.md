# Backend

Go backend содержит matching engine, HTTP-транспорт, подключение к PostgreSQL и
версионируемые миграции. Основная точка входа — `cmd/api`, служебная точка входа
для схемы данных — `cmd/migrate`.

## Локальный запуск без Docker для Go

Сначала запустите PostgreSQL/pgvector и примените миграции из корня репозитория:

```bash
docker compose up -d postgres
make migrate-up
make run-backend
```

По умолчанию backend слушает `:8080`, а локальное подключение использует
`postgres://swap_chain:swap_chain@127.0.0.1:5432/swap_chain?sslmode=disable`.
Настройки переопределяются переменными из корневого `.env.example`.

## Команды

Из директории `backend/` доступны эквиваленты целей Makefile:

```bash
go run ./cmd/migrate up
go run ./cmd/migrate down
go run ./cmd/migrate version
go run ./cmd/api
go test ./...
go build ./...
```

Миграция `000001` создаёт расширение pgvector, таблицы `users`, `items`,
`chains`, `chain_items`, enum-типы статусов, внешние ключи и индексы. Размерность
embedding зафиксирована как `vector(1024)` под текущую модель `bge-m3`.

Миграция `000005` добавляет метаданные анализа и индекс восстановления зависших
карточек. Полный pipeline оценивает описание, определяет категории и записывает
оба embedding одним атомарным обновлением перед переходом в `MATCHING`.

HTTP API предоставляет `GET /health`, который проверяет живое соединение с БД,
и `GET /items/{itemID}/matching`, запускающий существующий matching engine через
sqlc-запрос к PostgreSQL. Публичный контракт находится в `../api/openapi.yaml`.
