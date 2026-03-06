# fore-cast

Golf tee time booking automation. Books the earliest available slot before a configurable cutoff time, with support for retry loops, scheduled execution, push notifications, and a web UI.

Built and deployed on [Railway](https://railway.app).

## How it works

1. Logs in with your club member credentials.
2. Fetches available tee time slots for the target date (default: 1 week ahead).
3. Filters slots before the cutoff time (default: 8:15 AM).
4. Checks each slot's availability, then attempts to book the earliest one.

Course is selected automatically based on the day of the week (override with `-course`).

## Prerequisites

- Go 1.24.3+
- Docker (local Postgres + integration tests)
- Node.js 18+ and npm (Next.js frontend)
- Club member credentials
- A `.env.development` file at the project root (see [Configuration](#configuration))

## Getting started

```bash
# Start local Postgres
make db-up

# Run DB migrations
make db-migrate

# Start the web backend (loads .env.development automatically)
make web

# Start the frontend (separate terminal)
make ui
```

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

# Custom retry interval (e.g. 500ms, 1s)
./bin/fore-cast -user <member-id> -password <password> -retry -retry-interval 500ms

# Delay execution until a specific time (e.g. wait for booking window to open)
./bin/fore-cast -user <member-id> -password <password> -retry -at 22:00

# Enable push notifications via ntfy.sh
./bin/fore-cast -user <member-id> -password <password> -retry -ntfy <topic-name>
```

Run `./bin/fore-cast -help` for the full list of flags and options.

## Scheduler (production)

The scheduler reads all enabled presets from the database and runs the booking loop for each user. Designed to run as a cron job.

Presets are processed concurrently, capped at `MAX_CONCURRENT_PRESETS` (default 5). If the number of active users outgrows this, consider increasing the limit or moving to a queue-based architecture.

Run status (`running`, `success`, `failed`) is written back to the preset row so the web UI can display progress. Push notification topics are auto-generated per user when enabled via the settings page.

**Dry-run mode** (local testing only): run `make scheduler-dry` to mock the Booker API. Override with `make scheduler-dry SCENARIO=success` or `SCENARIO=empty`. No real HTTP calls are made.

## Cleanup service

A separate cron job that prunes booking history older than 30 days.

## Configuration

All configuration is loaded from environment variables using [env](https://github.com/caarlos0/env). Place a `.env.development` file at the project root for local dev (loaded automatically when `APP_ENV=development`).

Example `.env.development`:

```
DATABASE_URL=postgres://forecast:forecast@localhost:5432/forecast?sslmode=disable
ENCRYPTION_KEY=<output of: openssl rand -hex 32>
```

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | Environment name; loads `.env.<APP_ENV>` if present |
| `PORT` | `8080` | HTTP server port |
| `SESSION_TTL` | `24h` | Session time-to-live |
| `DATABASE_URL` | *(required)* | Postgres connection string |
| `ENCRYPTION_KEY` | *(required for presets)* | 64-char hex key for AES-256-GCM |
| `MAX_CONCURRENT_PRESETS` | `5` | Max presets the scheduler processes in parallel |
| `BOOKER_DRY_RUN` | `false` | Mock Booker API (local testing only) |
| `BOOKER_DRY_RUN_SCENARIO` | `timeout` | `success` \| `timeout` \| `empty` |
| `BOOKER_DRY_RUN_TIMEOUT` | *(none)* | Cap preset timeout when dry-run (e.g. `30s`) |

## Database migrations

Schema changes are managed by [golang-migrate](https://github.com/golang-migrate/migrate). SQL files live in `migrations/`.

```bash
make db-migrate       # apply pending migrations
make db-migrate-down  # roll back last migration
```

Migrations also run automatically on startup for the scheduler, cleanup, and web services.

## Development

```bash
make db-up         # start local Postgres
make db-down       # stop Postgres (data preserved)
make db-reset      # nuke volume and start fresh
make web           # run web server
make scheduler     # run scheduler (real Booker API)
make scheduler-dry # run scheduler in dry-run mode (mock API)
make cleanup       # run cleanup service (prune old history)
make fmt           # format Go code
make lint          # run golangci-lint
make test          # run tests (db tests require Docker)
make check         # fmt + lint + test
make generate      # regenerate mocks
```
