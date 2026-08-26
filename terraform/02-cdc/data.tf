# This stack owns no infrastructure of its own: it finds what 01-platform built
# by name, so neither stack has to read the other's state.
locals {
  bucket_name   = "${var.project_id}-cdc-lakehouse"
  dataset_id    = "cdc_ecommerce"
  connection_id = "cdc-lakehouse"

  # Created by the generator's cmd/setup, consumed by the stream
  # https://github.com/ManuelVenturaNeto/generic-data-generator
  publication_name = "ds_publication"
  replication_slot = "ds_replication_slot"
}

data "google_sql_database_instance" "db" {
  name = var.db_instance_name
}

data "google_bigquery_dataset" "cdc_ecommerce" {
  dataset_id = local.dataset_id
}

data "google_storage_bucket" "lakehouse" {
  name = local.bucket_name
}
