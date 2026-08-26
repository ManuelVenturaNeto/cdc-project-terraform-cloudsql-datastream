# The root of the dependency graph, in its own stack so that destroying it comes
# last: by then 02-cdc and 01-platform are already gone, and turning an API off
# can no longer interrupt a deletion still in flight.
locals {
  required_services = [
    "sqladmin.googleapis.com",
    "datastream.googleapis.com",
    "bigquery.googleapis.com",
    "bigqueryconnection.googleapis.com",
    "storage.googleapis.com",
  ]
}

resource "google_project_service" "required" {
  for_each = toset(local.required_services)

  project = var.project_id
  service = each.value

  disable_on_destroy = var.disable_on_destroy

  # Left at the default on purpose: if disabling fails because something else
  # depends on the API, that is the signal to leave it enabled, not to force it
  disable_dependent_services = false
}
