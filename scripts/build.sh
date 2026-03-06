#!/usr/bin/env bash
set -euo pipefail

# Lint and tests run in GitHub Actions. Railway builds after CI passes.
echo "==> Build binaries"
go build -o bin/fore-cast-scheduler ./cmd/scheduler
go build -o bin/fore-cast-cleanup  ./cmd/cleanup
go build -o bin/fore-cast-web      ./cmd/web
