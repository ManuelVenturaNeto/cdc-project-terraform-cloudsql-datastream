# data-generator

Workload generator for testing CDC (Datastream) against the Cloud SQL
`main_rent_movie` database. Five independent modules, one command each:

| Module | Command | What it does |
|---|---|---|
| 1. Schema | `go run -C data-generator ./cmd/setup` | Creates `users`, `movies`, `rentals`, plus the Datastream publication, replication slot and grants (idempotent) |
| 2. Initial load | `go run -C data-generator ./cmd/seed` | One-shot insert of random rows |
| 3. Inserter | `go run -C data-generator ./cmd/insert` | Inserts a random row every second, until Ctrl+C |
| 4. Updater | `go run -C data-generator ./cmd/update` | Updates a random row every second, until Ctrl+C |
| 5. Deleter | `go run -C data-generator ./cmd/delete` | Deletes a random row every second, until Ctrl+C |

## Setup

The database only has a private IP, so every module reaches it through an IAP
tunnel to one of the proxy VMs (kept open in a separate terminal):

```bash
gcloud auth application-default print-access-token > /tmp/adc-token
gcloud compute instances list --filter='name~cdc-proxy' \
  --project exemples-mini-projects --access-token-file /tmp/adc-token
gcloud compute start-iap-tunnel <proxy-vm-name> 5432 --local-host-port=localhost:5432 \
  --zone <proxy-vm-zone> --project exemples-mini-projects --access-token-file /tmp/adc-token
```

Copy `.env.example` to `.env` and set `DB_PASSWORD` to `db_password_postgres`
from `terraform/terraform.tfvars`. The other values stay as-is: the host is the
tunnel (`localhost`), and `DB_SSLMODE=disable` because TLS only starts at the
Cloud SQL Auth Proxy on the VM.

`setup` must run as a superuser (`postgres`) and before the second Terraform apply:
`terraform apply` → `./cmd/setup` → `terraform apply -var enable_stream=true`.

## Typical CDC test session

```bash
go run -C data-generator ./cmd/setup
go run -C data-generator ./cmd/seed -users 20 -movies 30 -rentals 50

# then, each in its own terminal, turn modules on as needed:
go run -C data-generator ./cmd/insert -interval 500ms    # 2 insert by second
go run -C data-generator ./cmd/update -interval 1s       # 1 update by a second
go run -C data-generator ./cmd/delete -interval 2s       # 2 deletes by a second
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
