locals {
  # Created by data-generator/cmd/setup, consumed by the stream
  publication_name = "ds_publication"
  replication_slot = "ds_replication_slot"
}

variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "region" {
  description = "Region for every regional resource"
  type        = string
  default     = "us-central1"
}

variable "proxy_zones" {
  description = "Zones the proxy MIG spreads across; must belong to var.region"
  type        = list(string)
  default     = ["us-central1-b", "us-central1-c"]
}

variable "proxy_instance_count" {
  description = "Proxy VMs behind the internal load balancer"
  type        = number
  default     = 2
}

variable "cloud_sql_proxy_image" {
  description = "Cloud SQL Auth Proxy container image"
  type        = string
  default     = "gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.14.0"
}

# The publication and the slot only exist after data-generator/cmd/setup runs
variable "enable_stream" {
  description = "Second apply: creates the source connection profile and the stream"
  type        = bool
  default     = false
}

variable "db_password_postgres" {
  description = "Database password for user postgres"
  type        = string
  sensitive   = true
}

variable "db_password_datastream" {
  description = "Database password for user datastream"
  type        = string
  sensitive   = true
}
