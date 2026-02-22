# Build the binary into bin/
BINARY := bin/alfred
WEB_BINARY := bin/alfred-web
CMD := ./cmd/cli
CMD_WEB := ./cmd/web

.PHONY: build build-web run run-web fmt lint
build:
	@mkdir -p bin
	go build -o $(BINARY) $(CMD)

build-web:
	@mkdir -p bin
	go build -o $(WEB_BINARY) $(CMD_WEB)

# Run the program
run:
	go run $(CMD)

run-web:
	go run $(CMD_WEB)

# Format Go code
fmt:
	go fmt ./...

# Lint Go code
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...
