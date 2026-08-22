# data-generator

Workload generator for testing CDC (Datastream) against the `main_ecommerce`
Postgres database. The domain is e-commerce delivery: users place orders that
are shipped to an address and tracked while in transit.

The generator talks plain Postgres over TCP, so it works unchanged against
**Cloud SQL** (this project) and **Amazon RDS** (a future one). Nothing in the
code knows about a cloud provider — the only difference is `DB_HOST` and
`DB_SSLMODE` in `.env`.

| Module | Command | What it does |
|---|---|---|
| 1. Migrations | `go run -C data-generator ./cmd/migrate` | Applies pending SQL migrations, tracked in `schema_migrations` |
| 2. CDC setup | `go run -C data-generator ./cmd/setup` | Datastream role grants, publication and replication slot |
| 3. Initial load | `go run -C data-generator ./cmd/seed` | One-shot insert of random rows |
| 4. Inserter | `go run -C data-generator ./cmd/insert` | Inserts a random row every second, until Ctrl+C |
| 5. Updater | `go run -C data-generator ./cmd/update` | Updates a random row every second, until Ctrl+C |
| 6. Deleter | `go run -C data-generator ./cmd/delete` | Deletes a random row every second, until Ctrl+C |

## Schema

Migration `0001_ecommerce_core`:

```
users(id, name, email UNIQUE, phone, created_at)
addresses(id, street, number, complement, district, city, state, zip_code, country, created_at)
user_addresses(user_id, address_id, label, is_default, created_at)   PK(user_id, address_id)
orders(id, status, total_amount, shipping_address_id, placed_at, updated_at)
user_orders(user_id, order_id, created_at)                           PK(user_id, order_id)
```

`orders.status` walks `placed → paid → shipped → in_transit → delivered`; the
updater advances one order per tick.

Migration `0002_order_location` is the "new feature" that lands mid-session:

```
order_location(id, order_id UNIQUE, latitude, longitude, recorded_at, updated_at)
```

One row per order, holding its current position while in transit. The inserter
creates it for `shipped`/`in_transit` orders, the updater nudges the
coordinates (a stream of UPDATE events, which is what makes it interesting for
CDC), and the deleter drops it once the order is `delivered`.

The running `insert`/`update`/`delete` processes re-check which tables exist
every 10 seconds, so applying `0002` does **not** require restarting them: they
start writing to `order_location` on their own.

## Connection pool

Every command opens exactly one `pgxpool` and reuses its connections for the
whole run — one query per tick never needs more than a couple of them. The pool
is configured explicitly instead of relying on pgx defaults (which size
`MaxConns` from the CPU count, and let the pool drain to zero):

| Variable | Default | Meaning |
|---|---|---|
| `DB_MAX_CONNS` | `10` | Hard ceiling of open connections |
| `DB_MIN_CONNS` | `2` | Kept warm, so ticks never pay a reconnect |
| `DB_CONN_MAX_LIFETIME` | `30m` | Recycles connections behind proxies/load balancers |
| `DB_CONN_MAX_IDLE_TIME` | `5m` | Releases the idle surplus above `DB_MIN_CONNS` |

Each command sets `application_name` to `data-generator/<command>`, so you can
see the pools from the database:

```sql
SELECT application_name, count(*), state
FROM pg_stat_activity
WHERE application_name LIKE 'data-generator/%'
GROUP BY 1, 3;
```

Keep `DB_MAX_CONNS` × number of running commands below the instance's
`max_connections` — on `db-f1-micro` that budget is small.

## Configuration

Copy `.env.example` to `.env`. Real environment variables always win over the
`.env` file, which is what lets RDS/Secrets Manager inject credentials without
touching the file. `DB_ENV_FILE` points at a different file.

| Variable | Cloud SQL (this project) | Amazon RDS |
|---|---|---|
| `DB_HOST` | `localhost` through the IAP tunnel, or the internal LB IP | the RDS endpoint |
| `DB_SSLMODE` | `disable` over the tunnel (TLS starts at the Auth Proxy) | `require` |
| `DB_NAME` | `main_ecommerce` | `main_ecommerce` |

### Cloud SQL access

The database only has a private IP, so every module reaches it through an IAP
tunnel to one of the proxy VMs (kept open in a separate terminal):

```bash
gcloud auth application-default print-access-token > /tmp/adc-token
gcloud compute instances list --filter='name~cdc-proxy' \
  --project exemples-mini-projects --access-token-file /tmp/adc-token
gcloud compute start-iap-tunnel <proxy-vm-name> 5432 --local-host-port=localhost:5432 \
  --zone <proxy-vm-zone> --project exemples-mini-projects --access-token-file /tmp/adc-token
```

Set `DB_PASSWORD` to `db_password_postgres` from `terraform/terraform.tfvars`.

## Migrations

SQL files live in `data-generator/migrations`, named `<version>_<name>.sql` and
embedded in the binary. Each one runs inside a transaction together with the
row that records it in `schema_migrations`, so a failure leaves nothing behind.

```bash
go run -C data-generator ./cmd/migrate -status   # what is applied, what is pending
go run -C data-generator ./cmd/migrate -to 1     # stop after 0001
go run -C data-generator ./cmd/migrate           # apply everything pending
```

Adding a migration is dropping a new `.sql` file in that folder.

## Typical CDC session

`setup` must run as a superuser (`postgres`), after the tables exist and before
the second Terraform apply:

```bash
terraform apply
go run -C data-generator ./cmd/migrate -to 1
go run -C data-generator ./cmd/setup
terraform apply -var enable_stream=true

go run -C data-generator ./cmd/seed -users 20 -addresses 30 -orders 50

# then, each in its own terminal, turn modules on as needed:
go run -C data-generator ./cmd/insert -interval 500ms    # 2 inserts per second
go run -C data-generator ./cmd/update -interval 1s       # 1 update per second
go run -C data-generator ./cmd/delete -interval 2s       # 1 delete every 2 seconds

# ship the new feature with everything still running:
go run -C data-generator ./cmd/migrate
```

The publication is `FOR ALL TABLES` and `setup` leaves an
`ALTER DEFAULT PRIVILEGES` grant behind, so `order_location` is picked up by
Datastream without re-running `setup`.

`insert`, `update` and `delete` accept:

- `-interval` — time between operations (default `1s`)
- `-table` — one table or `all` (default `all`); targets that do not exist yet
  are skipped with a log line instead of failing

Every operation logs its table and row id, so you can correlate the source of
each change with the events Datastream delivers.

Notes:

- Weights favor the tables that matter for the demo: orders on insert,
  `order_location` on update, orders and locations on delete.
- Deletes never break a foreign key. Users, addresses and orders are only
  removed once nothing references them, and their join rows go in the same
  transaction.
- Emails get a large random suffix to dodge the UNIQUE constraint; a rare
  collision is logged and skipped, never fatal.
