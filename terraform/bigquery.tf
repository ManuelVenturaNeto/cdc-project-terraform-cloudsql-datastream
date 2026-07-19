resource "google_bigquery_dataset" "cdc_movies" {
  dataset_id                 = "cdc_movies"
  friendly_name              = "cdc_movies"
  location                   = var.region
  delete_contents_on_destroy = true

  depends_on = [google_project_service.required]
}
