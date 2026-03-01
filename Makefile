# Load local dev config (.env.development is the single source of truth)
-include .env.development

# Build binaries into bin/
BINARY := bin/fore-cast
WEB_BINARY := bin/fore-cast-web
SCHEDULER_BINARY := bin/fore-cast-scheduler
CLEANUP_BINARY := bin/fore-cast-cleanup
CMD := ./cmd/cli
CMD_WEB := ./cmd/web
CMD_SCHEDULER := ./cmd/scheduler
CMD_CLEANUP := ./cmd/cleanup
MIGRATE := go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

.PHONY: build build-web build-scheduler build-cleanup build-all cli web fmt lint test check generate db-up db-down db-reset db-migrate db-migrate-down ui ui-install ui-build ui-lint
build:
	@mkdir -p bin
	go build -o $(BINARY) $(CMD)

build-web:
	@mkdir -p bin
	go build -o $(WEB_BINARY) $(CMD_WEB)

build-scheduler:
	@mkdir -p bin
	go build -o $(SCHEDULER_BINARY) $(CMD_SCHEDULER)

build-cleanup:
	@mkdir -p bin
	go build -o $(CLEANUP_BINARY) $(CMD_CLEANUP)

build-all: build build-web build-scheduler build-cleanup

# Run the CLI
cli:
	go run $(CMD)

# Run the web server (loads .env.development via APP_ENV)
web:
	APP_ENV=development go run $(CMD_WEB)

# Format Go code
fmt:
	go fmt ./...

# Lint Go code
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...

# Run Go tests
test:
	go test ./...

# Format, lint, and test (e.g. before commit or in CI)
check: fmt lint test

# Regenerate mocks. No global mockgen install required.
generate:
	go generate ./...

# Local Postgres via Docker
db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-reset:
	docker compose down -v
	docker compose up -d postgres

# Run pending DB migrations against the local Postgres
db-migrate:
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" up

db-migrate-down:
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" down 1

# UI: install deps, dev server, build, lint
ui-install:
	cd ui && npm install

# UI dev server: http://localhost:3000 (API backend should run on :8080, e.g. make web)
ui:
	cd ui && npm run dev

ui-build:
	cd ui && npm run build

ui-lint:
	cd ui && npm run lint
