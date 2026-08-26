# What the generator needs in its .env, and what 02-cdc looks up by name
output "postgres_host" {
  description = "Public IP of the instance; DB_HOST for the generator"
  value       = google_sql_database_instance.instance.public_ip_address
}

output "db_instance_name" {
  value = google_sql_database_instance.instance.name
}

output "lakehouse_bucket" {
  value = google_storage_bucket.lakehouse.name
}

output "bigquery_dataset" {
  value = google_bigquery_dataset.cdc_ecommerce.dataset_id
}
