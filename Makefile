# Build binaries into bin/
BINARY := bin/fore-cast
WEB_BINARY := bin/fore-cast-web
SCHEDULER_BINARY := bin/fore-cast-scheduler
CLEANUP_BINARY := bin/fore-cast-cleanup
CMD := ./cmd/cli
CMD_WEB := ./cmd/web
CMD_SCHEDULER := ./cmd/scheduler
CMD_CLEANUP := ./cmd/cleanup

.PHONY: build build-web build-scheduler build-cleanup build-all cli web fmt lint test check generate ui ui-install ui-build ui-lint
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

# Run the web server
web:
	go run $(CMD_WEB)

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
