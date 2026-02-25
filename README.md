# fore-cast

Golf tee time booking automation. Books the earliest available slot before a configurable cutoff time, with support for retry loops, scheduled execution, and specific slot targeting.

## How it works

1. Logs in with your club member credentials.
2. Fetches available tee time slots for the target date (default: 1 week ahead).
3. Filters slots before the cutoff time (default: 8:15 AM).
4. Checks each slot's availability, then attempts to book the earliest one.

Course is selected automatically based on the day of the week.

## Prerequisites

- Go 1.24.3+
- Club member credentials

## Build

```bash
make build        # builds binary to bin/fore-cast
```

## Usage

Credentials are passed via `-user` and `-password`.

```bash
# Book the earliest available slot (1 week ahead, before 8:15 AM)
./bin/fore-cast -user <member-id> -password <password>

# Check current bookings
./bin/fore-cast -user <member-id> -password <password> -status

# List available slots for a date
./bin/fore-cast -user <member-id> -password <password> -slots -date 2026/03/04

# Retry until booked (useful for competitive booking windows)
./bin/fore-cast -user <member-id> -password <password> -retry -at 22:00
```

Run `./bin/fore-cast -help` for the full list of flags and options.

## Development

```bash
make fmt       # format Go code
make lint      # run golangci-lint
make test      # run tests
make check     # fmt + lint + test
make generate  # regenerate mocks
```

