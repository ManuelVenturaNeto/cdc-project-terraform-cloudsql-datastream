data "http" "my_ip" {
  url = "https://api.ipify.org?format=text"
}

resource "google_sql_database" "database" {
    name = "main_movie"
    instance = google_sql_database_instance.instance.name
}

resource "google_sql_database_instance" "instance" {
    name                = "main-movie-0"
    region              = "us-central1"
    database_version    = "POSTGRES_16"
    deletion_protection = false

    settings {
        tier    = "db-f1-micro"
        edition = "ENTERPRISE"

        availability_type = "ZONAL"

        disk_size              = 10
        disk_autoresize         = true
        disk_autoresize_limit   = 30

        backup_configuration {
            enabled = false
        }

        password_validation_policy {
            min_length                   = 20
            complexity                   = "COMPLEXITY_DEFAULT"
            reuse_interval               = 30
            disallow_username_substring  = true
            password_change_interval    = "2592000s"
            enable_password_policy       = true
        }

        ip_configuration {
            ipv4_enabled = true
            ssl_mode     = "ENCRYPTED_ONLY"

            authorized_networks {
                name  = "wsl-dev"
                value = "${chomp(data.http.my_ip.response_body)}/32"
            }
        } 
    }

    lifecycle {
        ignore_changes = [settings[0].disk_size] # It can make some resize in production
    }
}

resource "google_sql_user" "postgres" {
    name     = "postgres"
    instance = google_sql_database_instance.instance.name
    password = var.db_password
}