# PSC interface instead of VPC peering: peering creation is broken in this project
resource "google_datastream_private_connection" "private" {
  display_name          = "datastream-vpc"
  location              = var.region
  private_connection_id = "datastream-vpc"

  psc_interface_config {
    network_attachment = google_compute_network_attachment.datastream.id
  }

  depends_on = [google_project_service.required]
}

resource "google_datastream_connection_profile" "destination" {
  display_name          = "bigquery"
  location              = var.region
  connection_profile_id = "bigquery"

  bigquery_profile {}

  depends_on = [google_project_service.required]
}

# Phase two: needs the publication and the slot from data-generator/cmd/setup
resource "google_datastream_connection_profile" "source" {
  count = var.enable_stream ? 1 : 0

  display_name          = "cloudsql-postgres"
  location              = var.region
  connection_profile_id = "cloudsql-postgres"

  # The hostname is the load balancer, not the database: Datastream only sees our VPC
  postgresql_profile {
    hostname = google_compute_forwarding_rule.proxy.ip_address
    port     = 5432
    username = google_sql_user.datastream.name
    password = var.db_password_datastream
    database = google_sql_database.database.name
  }

  private_connectivity {
    private_connection = google_datastream_private_connection.private.id
  }

  create_without_validation = false

  depends_on = [
    google_sql_database_instance.instance,
    google_sql_database.database,
    google_sql_user.datastream,
    google_datastream_private_connection.private,
    # validation opens TCP 5432 from the PSC interface against the load balancer
    google_compute_forwarding_rule.proxy,
    google_compute_firewall.allow_datastream_to_proxy,
  ]
}

resource "google_datastream_stream" "postgres_to_bigquery" {
  count = var.enable_stream ? 1 : 0

  stream_id     = "postgres-to-bigquery"
  display_name  = "postgres-to-bigquery"
  location      = var.region
  desired_state = "RUNNING" # without this the stream is created paused

  source_config {
    source_connection_profile = google_datastream_connection_profile.source[0].id

    postgresql_source_config {
      publication      = local.publication_name
      replication_slot = local.replication_slot
    }
  }

  destination_config {
    destination_connection_profile = google_datastream_connection_profile.destination.id

    bigquery_destination_config {
      data_freshness = "1s"

      append_only {}

      single_target_dataset {
        dataset_id = google_bigquery_dataset.cdc_movies.id
      }
    }
  }

  # Copies pre-existing table data before starting CDC
  backfill_all {}

  depends_on = [
    google_datastream_connection_profile.source,
    google_datastream_connection_profile.destination,
    google_bigquery_dataset.cdc_movies,
    google_compute_firewall.allow_datastream_to_proxy,
  ]
}
