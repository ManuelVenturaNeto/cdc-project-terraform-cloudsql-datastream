resource "google_compute_network" "vpc" {
  name                    = "cdc-vpc"
  auto_create_subnetworks = false

  depends_on = [google_project_service.required]
}

resource "google_compute_subnetwork" "subnet" {
  name          = "cdc-subnet"
  region        = var.region
  network       = google_compute_network.vpc.id
  ip_cidr_range = "10.0.0.0/24"

  # No public IPs on the proxies; this reaches gcr.io and the Google APIs
  private_ip_google_access = true

  depends_on = [google_compute_network.vpc]
}

# Private Services Access range: where Cloud SQL gets its private IP, via peering
resource "google_compute_global_address" "psa_range" {
  name          = "cloudsql-psa-range"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  address       = "10.1.0.0"
  prefix_length = 16
  network       = google_compute_network.vpc.id

  depends_on = [google_compute_network.vpc]
}

resource "google_service_networking_connection" "psa" {
  network                 = google_compute_network.vpc.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.psa_range.name]

  depends_on = [
    google_project_service.required,
    google_compute_network.vpc,
    google_compute_global_address.psa_range,
  ]
}

# Datastream's PSC interface gets an IP from cdc-subnet (see datastream.tf)
resource "google_compute_network_attachment" "datastream" {
  name                  = "datastream-psc"
  region                = var.region
  connection_preference = "ACCEPT_AUTOMATIC"
  subnetworks           = [google_compute_subnetwork.subnet.self_link]
}

resource "google_compute_firewall" "allow_datastream_to_proxy" {
  name    = "allow-datastream-to-proxy"
  network = google_compute_network.vpc.id

  allow {
    protocol = "tcp"
    ports    = ["5432"]
  }

  source_ranges = [google_compute_subnetwork.subnet.ip_cidr_range]
  target_tags   = ["cdc-proxy"]

  depends_on = [google_compute_network.vpc]
}

# Fixed probe ranges of the Google health checkers, required by the internal LB
resource "google_compute_firewall" "allow_health_checks" {
  name    = "allow-health-checks"
  network = google_compute_network.vpc.id

  allow {
    protocol = "tcp"
    ports    = ["5432"]
  }

  source_ranges = ["130.211.0.0/22", "35.191.0.0/16"]
  target_tags   = ["cdc-proxy"]

  depends_on = [google_compute_network.vpc]
}

# 35.235.240.0/20 is IAP's fixed range, for `gcloud compute start-iap-tunnel`
resource "google_compute_firewall" "allow_iap_tunnel" {
  name    = "allow-iap-tunnel"
  network = google_compute_network.vpc.id

  allow {
    protocol = "tcp"
    ports    = ["22", "5432"]
  }

  source_ranges = ["35.235.240.0/20"]
  target_tags   = ["cdc-proxy"]

  depends_on = [google_compute_network.vpc]
}
