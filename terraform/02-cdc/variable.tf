variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "region" {
  description = "Region for every regional resource"
  type        = string
  default     = "us-central1"
}

variable "db_instance_name" {
  description = "Cloud SQL instance 01-platform created; looked up by name"
  type        = string
  default     = "main-ecommerce"
}

variable "db_password_datastream" {
  description = "Password of the datastream user, as created by 01-platform"
  type        = string
  sensitive   = true
}
