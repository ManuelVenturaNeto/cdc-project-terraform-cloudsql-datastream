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
  description = "Cloud SQL instance name; 02-cdc looks the instance up by it"
  type        = string
  default     = "main-ecommerce"
}

variable "my_ip" {
  description = "Your public IP in CIDR form, so you can reach the database: curl ifconfig.me"
  type        = string
}

# Published at https://cloud.google.com/datastream/docs/ip-allowlists-and-regions
variable "datastream_ips" {
  description = "Datastream public IPs for var.region, in CIDR form"
  type        = list(string)
  default = [
    "34.72.28.29/32",
    "34.67.234.134/32",
    "34.67.6.157/32",
    "34.72.239.218/32",
    "34.71.242.81/32",
  ]
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
