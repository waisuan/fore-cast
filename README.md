# fore-cast

Golf tee time booking automation. Books the earliest available slot before a configurable cutoff time, with support for retry loops, scheduled execution, push notifications, and a web UI.

## Architecture

```
cmd/scheduler/  – production entry point, DB-driven (Railway cron)
cmd/cleanup/    – DB housekeeping (Railway cron), prunes stale records
cmd/cli/        – debug/scripting tool, flag-driven (no DB)
cmd/web/        – web backend + API server

internal/
  booker/       – external booking API client
  slotutil/     – slot filtering, date helpers
  runner/       – core booking loop (retry, slot selection, attempt)
  notify/       – ntfy.sh push notifications
  crypto/       – AES-256-GCM encrypt/decrypt for credentials
  db/           – Postgres service layer (interface + mock)
  deps/         – dependency injection (config, Postgres client, service wiring)
  handlers/     – HTTP handlers (bookings, history, presets)
  router/       – HTTP router
  session/      – in-memory session store
  context/      – request context helpers
  middlewares/  – CORS, session auth

migrations/     – versioned SQL migration files (golang-migrate)
ui/             – Next.js frontend
```

## How it works

1. Logs in with your club member credentials.
2. Fetches available tee time slots for the target date (default: 1 week ahead).
3. Filters slots before the cutoff time (default: 8:15 AM).
4. Checks each slot's availability, then attempts to book the earliest one.

Course is selected automatically based on the day of the week (override with `-course`).

## Prerequisites

- Go 1.24.3+
- Club member credentials
- PostgreSQL (required for scheduler, cleanup, and web — not needed for CLI)
- Docker (required for running `internal/db` integration tests via Testcontainers)
- Node.js 18+ and npm (for building the Next.js frontend)

## Build

```bash
make build            # builds CLI to bin/fore-cast
make build-web        # builds web server to bin/fore-cast-web
make build-scheduler  # builds scheduler to bin/fore-cast-scheduler
make build-cleanup    # builds cleanup to bin/fore-cast-cleanup
make build-all        # builds everything
```

## CLI usage (debug tool)

Credentials are passed via `-user` and `-password`. No database required.

```bash
# Book the earliest available slot (1 week ahead, before 8:15 AM)
./bin/fore-cast -user <member-id> -password <password>

# Check current bookings
./bin/fore-cast -user <member-id> -password <password> -status

# List available slots for a date
./bin/fore-cast -user <member-id> -password <password> -slots -date 2026/03/04

# Retry until booked (useful for competitive booking windows)
./bin/fore-cast -user <member-id> -password <password> -retry -timeout 10m

# Delay execution until a specific time (e.g. wait for booking window to open)
./bin/fore-cast -user <member-id> -password <password> -retry -at 22:00

# Enable push notifications via ntfy.sh
./bin/fore-cast -user <member-id> -password <password> -retry -ntfy <topic-name>
```

Run `./bin/fore-cast -help` for the full list of flags and options.

## Scheduler (production)

The scheduler reads all enabled presets from the database and runs the booking loop for each user. It's designed to run as a Railway cron job.

**Required environment variables:**

| Variable | Description |
|---|---|
| `DATABASE_URL` | Postgres connection string (auto-injected by Railway) |
| `ENCRYPTION_KEY` | 64-character hex string (32 bytes) for AES-256-GCM credential encryption |

Generate an encryption key:

```bash
openssl rand -hex 32
```

## Cleanup service

A separate cron job that prunes booking history older than 30 days. Only requires `DATABASE_URL`.

## Web server

The web backend serves the API for the Next.js frontend. Requires `DATABASE_URL` for history and auto-booker preset features.

```bash
# Run locally
make web

# With database
DATABASE_URL=postgres://... ENCRYPTION_KEY=... make web
```

## Configuration

All configuration is loaded from environment variables using [env](https://github.com/caarlos0/env). You can also place a `.env.<APP_ENV>` file at the project root (loaded via [godotenv](https://github.com/joho/godotenv)).

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | Environment name; loads `.env.<APP_ENV>` if present |
| `PORT` | `8080` | HTTP server port |
| `SESSION_SECRET` | `change-me-in-production` | Session signing secret |
| `SESSION_TTL` | `24h` | Session time-to-live |
| `DATABASE_URL` | *(empty)* | Postgres connection string |
| `ENCRYPTION_KEY` | *(empty)* | 64-char hex key for AES-256-GCM |

## Railway setup

Railway hosts all three backend services (scheduler, cleanup, web) from the same repo. Each service runs a different binary.

### 1. Create the Railway project

1. Provision a **Postgres** add-on (auto-injects `DATABASE_URL` into all services).
2. Set `ENCRYPTION_KEY` as a shared environment variable.

### 2. Deploy multiple services

Railway supports multiple services from the same GitHub repo. The `railway.toml` at the project root builds all binaries during the build step. Each service overrides its own `startCommand`.

| Service | Start command | Cron schedule | Restart policy |
|---|---|---|---|
| **Scheduler** | `bin/fore-cast-scheduler` | e.g. `55 21 * * *` (9:55 PM UTC) | Never |
| **Cleanup** | `bin/fore-cast-cleanup` | e.g. `0 4 * * *` (4:00 AM UTC daily) | Never |
| **Web** | `bin/fore-cast-web` | *(none — always on)* | On failure |

To set up each service:

1. In your Railway project, click **New** → **GitHub Repo** → select this repo.
2. In the service **Settings** tab, override the **Start Command** (e.g. `bin/fore-cast-cleanup` for the cleanup service).
3. For cron services (scheduler, cleanup): set the **Cron Schedule** in the service settings and set **Restart Policy** to "Never".
4. For the web service: leave the cron schedule empty and set restart policy to "On Failure".

All services share the same Postgres add-on and environment variables.

## Database migrations

Schema changes are managed by [golang-migrate](https://github.com/golang-migrate/migrate). SQL files live in `migrations/`.

Migrations run automatically on startup for the scheduler, cleanup, and web services. This is safe because:

- `golang-migrate` tracks applied versions in a `schema_migrations` table and skips already-applied migrations.
- The Postgres driver uses advisory locks to prevent concurrent migration runs, so multiple services starting simultaneously won't conflict.
- For destructive schema changes, always deploy backwards-compatible migrations first, then update application code in a follow-up deploy.

## Development

```bash
make fmt       # format Go code
make lint      # run golangci-lint
make test      # run tests (db tests require Docker)
make check     # fmt + lint + test
make generate  # regenerate mocks
```
