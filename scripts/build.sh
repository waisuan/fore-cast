#!/usr/bin/env bash
set -euo pipefail

echo "==> Lint"
go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...

echo "==> Test"
go test ./...

echo "==> Build binaries"
go build -o bin/fore-cast-scheduler ./cmd/scheduler
go build -o bin/fore-cast-cleanup  ./cmd/cleanup
go build -o bin/fore-cast-web      ./cmd/web
