.DEFAULT_GOAL := help

.PHONY: help config up down logs lint lint-go test

help: ## Show available commands
	@echo Usage: make target
	@echo Targets:
	@echo   help         Show available commands
	@echo   config       Validate Docker Compose configuration
	@echo   up           Start PostgreSQL and Swagger UI
	@echo   down         Stop services without deleting database data
	@echo   logs         Follow service logs
	@echo   lint         Run all currently available linters
	@echo   lint-go      Run golangci-lint for the backend
	@echo   test         Run tests for initialized projects

config: ## Validate and render Docker Compose configuration
	docker compose config --quiet

up: ## Start local PostgreSQL and Swagger UI
	docker compose up -d

down: ## Stop local services without deleting database data
	docker compose down

logs: ## Follow logs from local services
	docker compose logs -f

lint: lint-go ## Run all currently available linters

lint-go: ## Run golangci-lint for the backend when its Go module exists
ifeq ($(wildcard backend/go.mod),)
	@echo Go lint skipped because backend/go.mod is not present
else
	@cd backend && golangci-lint run ./...
endif

test: ## Run tests for initialized backend and frontend projects
ifeq ($(wildcard backend/go.mod),)
	@echo Go tests skipped because backend/go.mod is not present
else
	@cd backend && go test ./...
endif
ifeq ($(wildcard frontend/package.json),)
	@echo Frontend tests skipped because frontend/package.json is not present
else
	@cd frontend && npm test -- --run
endif
