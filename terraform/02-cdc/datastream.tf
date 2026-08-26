resource "google_datastream_connection_profile" "destination" {
  display_name          = "iceberg"
  location              = var.region
  connection_profile_id = "iceberg"

  bigquery_profile {}
}

# Datastream reaches the instance over its public IP, from the fixed addresses
# 01-platform put in the allowlist. It still has to prove who it is talking to,
# which is what the server certificate below is for.
resource "google_datastream_connection_profile" "source" {
  display_name          = "cloudsql-postgres"
  location              = var.region
  connection_profile_id = "cloudsql-postgres"

  postgresql_profile {
    hostname = data.google_sql_database_instance.db.public_ip_address
    port     = 5432
    username = "datastream"
    password = var.db_password_datastream
    database = "main_ecommerce"

    ssl_config {
      server_verification {
        ca_certificate = data.google_sql_database_instance.db.server_ca_cert[0].cert
      }
    }
  }
}

# The publication and the slot have to exist before this applies: run the
# generator's cmd/setup against the database first
resource "google_datastream_stream" "postgres_to_iceberg" {
  stream_id     = "postgres-to-iceberg"
  display_name  = "postgres-to-iceberg"
  location      = var.region
  desired_state = "RUNNING" # without this the stream is created paused

  source_config {
    source_connection_profile = google_datastream_connection_profile.source.id

    postgresql_source_config {
      publication      = local.publication_name
      replication_slot = local.replication_slot
    }
  }

  destination_config {
    destination_connection_profile = google_datastream_connection_profile.destination.id

    bigquery_destination_config {
      data_freshness = "1s"

      # Iceberg destinations only support append-only: every change arrives as a
      # new row, and the current state of a table is a view over the newest
      # datastream_metadata.sort_keys per primary key
      append_only {}

      # Turns the tables Datastream creates in the dataset into BigLake Iceberg
      # tables, stored in our own bucket instead of BigQuery storage
      blmt_config {
        bucket          = data.google_storage_bucket.lakehouse.name
        root_path       = "cdc_ecommerce"
        connection_name = "${var.project_id}.${var.region}.${local.connection_id}"
        file_format     = "PARQUET"
        table_format    = "ICEBERG"
      }

      single_target_dataset {
        dataset_id = data.google_bigquery_dataset.cdc_ecommerce.id
      }
    }
  }

  # Copies pre-existing table data before starting CDC
  backfill_all {}
}
