# terraform

Postgres on Cloud SQL, streamed by Datastream into an Apache Iceberg table whose
files live in a Cloud Storage bucket of ours.

Three stacks, each with its own state. They do not read each other's state: a
stack finds what the previous one built by name, so the names are the contract
between them (`db_instance_name`, `<project_id>-cdc-lakehouse`, `cdc_ecommerce`,
`cdc-lakehouse`).

```
00-apis/       enables the five APIs everything else needs
01-platform/   Cloud SQL instance, database, users; bucket, dataset, connection
02-cdc/        Datastream connection profiles and the stream
```

Copy `terraform.tfvars.example` to `terraform.tfvars` in each stack. Values that
appear in more than one file — the project, the region, the instance name, the
`datastream` password — have to match.

## Apply

Order matters, and the steps in between are yours to run:

```bash
cd 00-apis     && terraform init && terraform apply
cd 01-platform && terraform init && terraform apply
```

`01-platform` prints the public IP of the instance. Point the generator at it
(`DB_HOST`, `DB_SSLMODE=require`) and prepare the database from your machine —
<https://github.com/ManuelVenturaNeto/generic-data-generator>:

```bash
go run ./cmd/migrate -to 1   # only 0001; the new table ships later
go run ./cmd/setup           # publication ds_publication, slot ds_replication_slot
go run ./cmd/seed
```

The stream cannot be created before that: `02-cdc` names a publication and a
replication slot that `cmd/setup` is what creates.

```bash
cd 02-cdc && terraform init && terraform apply
```

Then start the loops (`insert`, `update`, `delete`) and apply migration `0002`
mid-session to watch a brand new table show up in the destination.

Two things about the first apply: enabling an API takes a few seconds longer
than `00-apis` takes to return, so if `01-platform` says an API is not enabled,
just run it again. And `my_ip` is your current public IP — if your connection
changes address, re-apply `01-platform` to update the allowlist.

## Destroy

Reverse order. `02-cdc` first, because `01-platform` owns the resources its data
sources look up:

```bash
cd 02-cdc      && terraform destroy
cd 01-platform && terraform destroy
cd 00-apis     && terraform destroy   # -var disable_on_destroy=true to also turn the APIs off
```

By default the APIs stay enabled, which costs nothing: you pay for resources, not
for an API being on. Disabling them affects the whole project, not only this
demo, so it is opt-in.

Destroying `02-cdc` deletes the stream but leaves `ds_replication_slot` behind in
Postgres, holding WAL. If you are keeping the database, drop it yourself:

```sql
SELECT pg_drop_replication_slot('ds_replication_slot');
```

The bucket has `force_destroy` and the dataset has `delete_contents_on_destroy`,
so destroy does not stop halfway on Parquet files or on tables Datastream
created — the two things that would otherwise keep billing after you thought the
project was clean.

Cloud SQL blocks reuse of an instance name for up to a week. After a destroy,
bump `db_instance_name` in `01-platform` and `02-cdc` before applying again.

## Notes

The database has a public IP with an allowlist — your machine and the five
published Datastream addresses for the region — instead of a VPC with proxy VMs
behind an internal load balancer. The subject here is CDC, not networking. TLS
stays mandatory (`ssl_mode = ENCRYPTED_ONLY`) and the stream verifies the server
certificate; only the password is deliberately weak.

The destination is append-only, which is the only mode Iceberg tables support.
Every change lands as a new row, so the current state of a table is a view over
the newest change per key:

```sql
SELECT * FROM `cdc_ecommerce.orders`
QUALIFY ROW_NUMBER() OVER (
  PARTITION BY id ORDER BY datastream_metadata.sort_keys DESC
) = 1
```
