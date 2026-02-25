# Build the binary into bin/
BINARY := bin/fore-cast
WEB_BINARY := bin/fore-cast-web
CMD := ./cmd/cli
CMD_WEB := ./cmd/web

.PHONY: build build-web build-all cli web fmt lint test check generate ui ui-install ui-build ui-lint
build:
	@mkdir -p bin
	go build -o $(BINARY) $(CMD)

build-web:
	@mkdir -p bin
	go build -o $(WEB_BINARY) $(CMD_WEB)

build-all: build build-web

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
