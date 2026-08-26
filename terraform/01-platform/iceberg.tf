locals {
  bucket_name   = "${var.project_id}-cdc-lakehouse"
  dataset_id    = "cdc_ecommerce"
  connection_id = "cdc-lakehouse"
}

resource "google_storage_bucket" "lakehouse" {
  name                        = local.bucket_name
  location                    = var.region
  uniform_bucket_level_access = true

  force_destroy = true
}

resource "google_bigquery_dataset" "cdc_ecommerce" {
  dataset_id    = local.dataset_id
  friendly_name = local.dataset_id
  location      = var.region

  delete_contents_on_destroy = true

  depends_on = [google_storage_bucket.lakehouse]
}

resource "google_bigquery_connection" "lakehouse" {
  connection_id = local.connection_id
  location      = var.region
  friendly_name = local.connection_id
  description   = "Cloud resource connection BigQuery uses to write the Iceberg files"

  cloud_resource {}
}

resource "google_storage_bucket_iam_member" "lakehouse_writer" {
  bucket = google_storage_bucket.lakehouse.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_bigquery_connection.lakehouse.cloud_resource[0].service_account_id}"
}
