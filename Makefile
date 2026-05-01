# Load local dev config (.env.development is the single source of truth)
-include .env.development

# Build binaries into bin/
WEB_BINARY := bin/fore-cast-web
SCHEDULER_BINARY := bin/fore-cast-scheduler
CLEANUP_BINARY := bin/fore-cast-cleanup
CMD_WEB := ./cmd/web
CMD_SCHEDULER := ./cmd/scheduler
CMD_CLEANUP := ./cmd/cleanup
CMD_CREATEUSER := ./cmd/createuser
MIGRATE := go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

.PHONY: build-web build-scheduler build-cleanup build-all web scheduler scheduler-dry cleanup fmt lint test check generate db-up db-down db-reset db-migrate db-migrate-down create-user ui ui-mock ui-mock-admin ui-install ui-build ui-lint e2e e2e-install
build-web:
	@mkdir -p bin
	go build -o $(WEB_BINARY) $(CMD_WEB)

build-scheduler:
	@mkdir -p bin
	go build -o $(SCHEDULER_BINARY) $(CMD_SCHEDULER)

build-cleanup:
	@mkdir -p bin
	go build -o $(CLEANUP_BINARY) $(CMD_CLEANUP)

build-all: build-web build-scheduler build-cleanup

# Run the web server (loads .env.development via APP_ENV). Dry-run is on by
# default so local usage never hits the real club API; override with
# BOOKER_DRY_RUN=false make web for a real-backend smoke test.
web:
	BOOKER_DRY_RUN=$(or $(BOOKER_DRY_RUN),true) APP_ENV=development go run $(CMD_WEB)

# Run the scheduler (loads .env.development via APP_ENV)
scheduler:
	APP_ENV=development go run $(CMD_SCHEDULER)

# Run the scheduler in dry-run mode (mock Booker API, no real HTTP calls)
# Override: make scheduler-dry SCENARIO=success  or  make scheduler-dry SCENARIO=empty
scheduler-dry:
	BOOKER_DRY_RUN=true BOOKER_DRY_RUN_SCENARIO=$(or $(SCENARIO),timeout) BOOKER_DRY_RUN_TIMEOUT=$(or $(TIMEOUT),30s) APP_ENV=development go run $(CMD_SCHEDULER)

# Run the cleanup service (prunes old history rows)
cleanup:
	APP_ENV=development go run $(CMD_CLEANUP)

# Format Go code
fmt:
	go fmt ./...

# Lint Go code
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...

# Run Go tests (includes integration tests; requires Docker)
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

# Seed or update a local test account. ROLE defaults to NON_ADMIN.
#   make create-user USER=alice PASS=secret
#   make create-user USER=admin PASS=admin ROLE=ADMIN
create-user:
	@test -n "$(USER)" || (echo "USER is required (e.g. make create-user USER=alice PASS=secret)" && exit 2)
	@test -n "$(PASS)" || (echo "PASS is required (e.g. make create-user USER=alice PASS=secret)" && exit 2)
	APP_ENV=development go run $(CMD_CREATEUSER) -user $(USER) -pass $(PASS) -role $(or $(ROLE),NON_ADMIN)

# UI: install deps, dev server, build, lint
ui-install:
	cd ui && npm install

# UI dev server: http://localhost:3000 (API backend should run on :8080, e.g. make web)
ui:
	cd ui && npm run dev

# UI dev with mocked /api/v1 (no Go server). See ui/README.md
ui-mock:
	cd ui && npm run dev:mock

# Same, mock user role ADMIN (admin shell at /admin/users)
ui-mock-admin:
	cd ui && npm run dev:mock:admin

ui-build:
	cd ui && npm run build

ui-lint:
	cd ui && npm run lint

# E2E: Playwright + Next dev (mocked /api in tests). Starts dev server via playwright.config.
e2e-install:
	cd ui && npm run test:e2e:install

e2e:
	cd ui && npm run test:e2e
