resource "google_bigquery_dataset" "cdc_ecommerce" {
  dataset_id                 = "cdc_ecommerce"
  friendly_name              = "cdc_ecommerce"
  location                   = var.region
  delete_contents_on_destroy = true

  depends_on = [google_project_service.required]
}
