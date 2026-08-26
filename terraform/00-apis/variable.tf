variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "region" {
  description = "Region for every regional resource"
  type        = string
  default     = "us-central1"
}

variable "disable_on_destroy" {
  description = <<-EOT
    Whether `terraform destroy` also turns the APIs back off. Leave false in a
    project shared with other work: disabling an API affects everything in the
    project, not just this stack. Enabling an API costs nothing on its own.
  EOT
  type        = bool
  default     = false
}
