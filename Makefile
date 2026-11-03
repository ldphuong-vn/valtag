.PHONY: help dev down build test test-unit test-integration test-dashboard test-mcp lint migrate-up migrate-down seed clean security rotate-key

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev: ## Start dev environment
	docker compose up -d

down: ## Stop all services
	docker compose down

build: ## Build all services
	docker compose build

test-unit: ## Run Go unit tests (no DB)
	cd server && go test ./internal/... ./pkg/... -v -count=1

test-integration: ## Run Go integration tests against a real Postgres (DATABASE_URL, falls back to the compose default)
	cd server && DATABASE_URL=$${DATABASE_URL:-postgres://valt:valtag_dev_password@localhost:5432/valt?sslmode=disable} go test ./internal/... ./pkg/... -count=1 -timeout 10m

test-dashboard: ## Run dashboard tests
	cd dashboard && npm test

test-mcp: ## Run Rust MCP tests
	cd mcp-server && cargo test

test: test-unit test-dashboard test-mcp ## Run all tests (unit + dashboard + mcp)

lint: ## Run all linters
	cd server && golangci-lint run ./...
	cd dashboard && npm run lint
	cd mcp-server && cargo clippy -- -D warnings

security: ## Run trivy security scan
	trivy fs --severity HIGH,CRITICAL .

migrate-up: ## Run database migrations
	docker compose run --rm server valtag-migrate up

migrate-down: ## Rollback last migration
	docker compose run --rm server valtag-migrate down

rotate-key: ## Re-wrap all master-key-encrypted data to a new key (server STOPPED; env OLD_MASTER_KEY + NEW_MASTER_KEY required)
	docker compose run --rm -e OLD_MASTER_KEY=$${OLD_MASTER_KEY} -e NEW_MASTER_KEY=$${NEW_MASTER_KEY} server valtag-rotate-master-key

seed: ## Seed development data
	docker compose run --rm server valtag-seed

logs: ## Tail all service logs
	docker compose logs -f

logs-server: ## Tail server logs
	docker compose logs -f server

ps: ## Show running services
	docker compose ps

clean: ## Remove all volumes and containers
	docker compose down -v --remove-orphans

restart: ## Restart all services
	docker compose restart

db-shell: ## Open psql shell
	docker compose exec postgres psql -U $${POSTGRES_USER:-valt} -d $${POSTGRES_DB:-valt}

