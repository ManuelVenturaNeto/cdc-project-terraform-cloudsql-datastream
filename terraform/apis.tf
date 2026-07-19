locals {
  # The real root of the graph: on a fresh project everything fails without these
  required_services = [
    "compute.googleapis.com",
    "servicenetworking.googleapis.com",
    "sqladmin.googleapis.com",
    "datastream.googleapis.com",
    "bigquery.googleapis.com",
    "iam.googleapis.com",
  ]
}

resource "google_project_service" "required" {
  for_each = toset(local.required_services)

  project = var.project_id
  service = each.value

  # Disabling on destroy can wedge the destroy while resources still propagate
  disable_on_destroy = false
}
