# Home and office addresses change; detecting it on every plan means the allowlist
# follows you, and re-applying after a reconnect is what fixes access
data "http" "my_ip" {
  count = var.my_ip == null ? 1 : 0

  url = "https://api.ipify.org"

  lifecycle {
    postcondition {
      condition     = can(regex("^[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+$", chomp(self.response_body)))
      error_message = "Could not detect a public IP. Set my_ip in terraform.tfvars instead."
    }
  }
}

locals {
  my_ip_cidr = var.my_ip != null ? var.my_ip : "${chomp(data.http.my_ip[0].response_body)}/32"

  authorized_networks = merge(
    { "local-machine" = local.my_ip_cidr },
    { for index, cidr in var.datastream_ips : "datastream-${index}" => cidr },
  )
}

resource "google_sql_database_instance" "instance" {
  name                = var.db_instance_name
  region              = var.region
  database_version    = "POSTGRES_16"
  deletion_protection = false

  settings {

    edition   = "ENTERPRISE"
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

  # DROP ROLE fails once the role owns tables or has grants, which is exactly
  # what the generator leaves behind. Deleting the instance removes the user
  # anyway, so destroy just forgets it
  deletion_policy = "ABANDON"
}

# Read-only user Datastream logs in as; cmd/setup grants it REPLICATION and SELECT
resource "google_sql_user" "datastream" {
  name     = "datastream"
  instance = google_sql_database_instance.instance.name
  password = var.db_password_datastream

  deletion_policy = "ABANDON"
}
