# fore-cast

Golf tee time booking automation. Books the earliest available slot before a configurable cutoff time, with support for retry loops, scheduled execution, push notifications, and a web UI.

Built and deployed on [Railway](https://railway.app).

## How it works

1. Logs in with your club member credentials.
2. Fetches available tee time slots for the target date (default: 1 week ahead).
3. Filters slots before the cutoff time (default: 8:15 AM).
4. Walks those slots in time order: checks each slot's status, then attempts at most one book per slot (skipping when the API says the flight is already reserved).
5. Repeats full passes until a booking succeeds, every slot is already reserved, or the preset timeout elapses. The retry interval is the pause **between** full passes, not between individual slots.

Course is selected automatically based on the day of the week (configurable per preset in the web UI).

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

**UI only (mock API):** `make ui-mock` (or `cd ui && npm run dev:mock`) — see `ui/README.md`.

## Build

```bash
make build-web        # builds web server to bin/fore-cast-web
make build-scheduler  # builds scheduler to bin/fore-cast-scheduler
make build-cleanup    # builds cleanup to bin/fore-cast-cleanup
make build-all        # builds everything
```

## Scheduler (production)

The scheduler reads all enabled presets from the database and runs the booking loop for each user. Designed to run as a cron job.

Presets are processed concurrently, capped at `MAX_CONCURRENT_PRESETS` (default 5). If the number of active users outgrows this, consider increasing the limit or moving to a queue-based architecture.

Run status (`running`, `success`, `failed`) is written back to the preset row so the web UI can display progress. Push notification topics are auto-generated per user when enabled via the settings page.

**Local testing**: run `make scheduler` to execute against the real Booker API (requires enabled presets in the web UI). Use `make scheduler-dry` to mock the API—override with `make scheduler-dry SCENARIO=success` or `SCENARIO=empty`. No real HTTP calls are made in dry-run.

## Admin registration

Admins can register new users at `/admin/register`. Set `ADMIN_USER` and `ADMIN_PASSWORD` to enable. The page requires these credentials (HTTP Basic Auth). Registration validates the new user's 3rd party credentials and creates a row in `user_credentials` only. The user can then log in and configure their preset in Settings.

**Login** uses stored credentials only (no 3rd party call). **Slots, bookings, and booking** obtain a 3rd party token on-demand when the user accesses those features.

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
| `SCHEDULER_TXN_DATE` | *(none)* | Override target date (YYYY/MM/DD); empty = 1 week ahead. Useful for testing. |
| `BOOKER_DRY_RUN` | `false` | Mock Booker API (local testing only) |
| `BOOKER_DRY_RUN_SCENARIO` | `timeout` | `success` \| `timeout` \| `empty` |
| `BOOKER_DRY_RUN_TIMEOUT` | *(none)* | Cap preset timeout when dry-run (e.g. `30s`) |
| `ADMIN_USER` | *(none)* | Admin username for `/admin/register` (Basic Auth) |
| `ADMIN_PASSWORD` | *(none)* | Admin password for `/admin/register` (Basic Auth) |

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

## Testing

- **Go:** `make test` (integration tests need Docker / Postgres as noted in [Prerequisites](#prerequisites)).
- **Frontend:** Vitest + Playwright — see [ui/README.md](ui/README.md#testing).
