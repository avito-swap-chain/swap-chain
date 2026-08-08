.DEFAULT_GOAL := help

.PHONY: help config build run-backend up down logs migrate-up migrate-down migrate-version lint lint-go test test-go

ifeq ($(OS),Windows_NT)
CLEAR_GOROOT := set GOROOT=&&
else
CLEAR_GOROOT := unset GOROOT;
endif

help: ## Show available commands
	@echo Usage: make target
	@echo Targets:
	@echo   help            Show available commands
	@echo   config          Validate Docker Compose configuration
	@echo   build           Build all backend commands
	@echo   run-backend     Run backend against the configured database
	@echo   up              Build and start database, migrations, backend, and Swagger UI
	@echo   down            Stop services without deleting database data
	@echo   logs            Follow service logs
	@echo   migrate-up      Apply all pending database migrations
	@echo   migrate-down    Roll back one database migration
	@echo   migrate-version Print the current migration version
	@echo   lint            Run all currently available linters
	@echo   lint-go         Run golangci-lint for the backend
	@echo   test            Run all initialized project tests
	@echo   test-go         Run backend tests

config: ## Validate and render Docker Compose configuration
	docker compose config --quiet

build: ## Build all backend commands
	$(CLEAR_GOROOT) cd backend && go build ./...

run-backend: ## Run backend against the configured database
	$(CLEAR_GOROOT) cd backend && go run ./cmd/api

up: ## Start the complete local environment
	docker compose up -d --build

down: ## Stop local services without deleting database data
	docker compose down

logs: ## Follow service logs
	docker compose logs -f

migrate-up: ## Apply all pending migrations to DATABASE_URL
	$(CLEAR_GOROOT) cd backend && go run ./cmd/migrate up

migrate-down: ## Roll back one migration from DATABASE_URL
	$(CLEAR_GOROOT) cd backend && go run ./cmd/migrate down

migrate-version: ## Print the current migration version
	$(CLEAR_GOROOT) cd backend && go run ./cmd/migrate version

lint: lint-go ## Run all currently available linters

lint-go: ## Run golangci-lint for the backend when its Go module exists
ifeq ($(wildcard backend/go.mod),)
	@echo Go lint skipped because backend/go.mod is not present
else
	@$(CLEAR_GOROOT) cd backend && golangci-lint run ./...
endif

test: test-go ## Run tests for initialized backend and frontend projects
ifeq ($(wildcard frontend/package.json),)
	@echo Frontend tests skipped because frontend/package.json is not present
else
	@cd frontend && npm test -- --run
endif

test-go: ## Run backend tests
ifeq ($(wildcard backend/go.mod),)
	@echo Go tests skipped because backend/go.mod is not present
else
	@$(CLEAR_GOROOT) cd backend && go test ./...
endif
