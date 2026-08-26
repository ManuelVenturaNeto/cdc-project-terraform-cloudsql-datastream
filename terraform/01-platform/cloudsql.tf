# The instance has a public IP and an allowlist instead of a VPC with proxy VMs:
# the subject here is CDC, not networking. Two kinds of client reach it — this
# machine, and Datastream's fixed public IPs.
locals {
  authorized_networks = merge(
    { "local-machine" = var.my_ip },
    { for index, cidr in var.datastream_ips : "datastream-${index}" => cidr },
  )
}

resource "google_sql_database_instance" "instance" {
  name                = var.db_instance_name
  region              = var.region
  database_version    = "POSTGRES_16"
  deletion_protection = false

  settings {
    tier      = "db-f1-micro"
    disk_size = 10

    backup_configuration {
      enabled = false
    }

    ip_configuration {
      ipv4_enabled = true

      # TLS stays mandatory: the database is on the internet, only the password is weak
      ssl_mode = "ENCRYPTED_ONLY"

      dynamic "authorized_networks" {
        for_each = local.authorized_networks

        content {
          name  = authorized_networks.key
          value = authorized_networks.value
        }
      }
    }

    database_flags {
      name  = "cloudsql.logical_decoding" # enables logical WAL, the source of CDC
      value = "on"
    }
  }
}

resource "google_sql_database" "database" {
  name     = "main_ecommerce"
  instance = google_sql_database_instance.instance.name
}

# Admin user: the one that runs the migrations and cmd/setup from the generator
resource "google_sql_user" "postgres" {
  name     = "postgres"
  instance = google_sql_database_instance.instance.name
  password = var.db_password_postgres
}

# Read-only user Datastream logs in as; cmd/setup grants it REPLICATION and SELECT
resource "google_sql_user" "datastream" {
  name     = "datastream"
  instance = google_sql_database_instance.instance.name
  password = var.db_password_datastream
}
