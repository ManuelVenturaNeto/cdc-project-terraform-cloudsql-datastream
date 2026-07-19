# Cloud SQL blocks name reuse for up to a week, so destroy/apply needs a fresh name
resource "random_id" "db_suffix" {
  byte_length = 2
}

resource "google_sql_database_instance" "instance" {
  name                = "main-rent-movie-${random_id.db_suffix.hex}"
  region              = var.region
  database_version    = "POSTGRES_16"
  deletion_protection = false

  settings {
    tier    = "db-f1-micro"
    edition = "ENTERPRISE"

    availability_type = "ZONAL"

    disk_size             = 10
    disk_autoresize       = true
    disk_autoresize_limit = 30

    backup_configuration {
      enabled = false
    }

    password_validation_policy {
      min_length                  = 20
      complexity                  = "COMPLEXITY_DEFAULT"
      reuse_interval              = 30
      disallow_username_substring = true
      password_change_interval    = "2592000s"
      enable_password_policy      = true
    }

    ip_configuration {
      ipv4_enabled    = false
      ssl_mode        = "ENCRYPTED_ONLY"
      private_network = google_compute_network.vpc.id
    }

    database_flags {
      name  = "cloudsql.logical_decoding" # enables logical WAL, the source of CDC
      value = "on"
    }
  }

  lifecycle {
    ignore_changes = [settings[0].disk_size] # It can make some resize in production
  }

  depends_on = [
    google_project_service.required,
    google_compute_network.vpc,
    # the private IP can only be allocated after the Cloud SQL peering exists
    google_service_networking_connection.psa,
    random_id.db_suffix,
  ]
}

resource "google_sql_database" "database" {
  name     = "main_rent_movie"
  instance = google_sql_database_instance.instance.name

  depends_on = [google_sql_database_instance.instance]
}

resource "google_sql_user" "postgres" {
  name     = "postgres"
  instance = google_sql_database_instance.instance.name
  password = var.db_password_postgres

  depends_on = [
    google_sql_database_instance.instance,
    google_sql_database.database,
  ]
}

resource "google_sql_user" "datastream" {
  name     = "datastream"
  instance = google_sql_database_instance.instance.name
  password = var.db_password_datastream

  # REPLICATION and the SELECT grants are applied by the VM bootstrap
  depends_on = [
    google_sql_database_instance.instance,
    google_sql_database.database,
  ]
}
