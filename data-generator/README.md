# data-generator

Workload generator for testing CDC (Datastream) against the Cloud SQL
`main_movie` database. Five independent modules, one command each:

| Module | Command | What it does |
|---|---|---|
| 1. Schema | `go run -C data-generator ./cmd/setup` | Creates `users`, `movies`, `rentals` (idempotent) |
| 2. Initial load | `go run -C data-generator ./cmd/seed` | One-shot insert of random rows |
| 3. Inserter | `go run -C data-generator ./cmd/insert` | Inserts a random row every second, until Ctrl+C |
| 4. Updater | `go run -C data-generator ./cmd/update` | Updates a random row every second, until Ctrl+C |
| 5. Deleter | `go run -C data-generator ./cmd/delete` | Deletes a random row every second, until Ctrl+C |

## Setup

Copy `.env.example` to `.env` and fill in the real values (same variables the
old movie-go API used).

## Typical CDC test session

```bash
go run -C data-generator ./cmd/setup
go run -C data-generator ./cmd/seed -users 20 -movies 30 -rentals 50

# then, each in its own terminal, turn modules on as needed:
go run -C data-generator ./cmd/insert                  # random table, 1/s
go run -C data-generator ./cmd/update -interval 2s     # slower updates
go run -C data-generator ./cmd/delete -table rentals   # deletes only rentals
```

`insert`, `update` and `delete` accept:

- `-interval` — time between operations (default `1s`)
- `-table` — `users`, `movies`, `rentals` or `all` (default `all`)

Every operation is logged with table and row id, so you can correlate the
source of each change with the events Datastream delivers.

Notes:

- The deleter only removes users/movies that no rental references, so foreign
  keys never fail; rentals are deleted more often on `all`.
- Emails get a large random suffix to dodge the UNIQUE constraint; a rare
  collision is logged and skipped, never fatal.
