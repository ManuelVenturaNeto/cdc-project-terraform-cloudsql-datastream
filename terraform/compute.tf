resource "google_service_account" "cdc_proxy" {
  account_id   = "cdc-proxy"
  display_name = "CDC proxy VM"

  depends_on = [google_project_service.required]
}

resource "google_project_iam_member" "cdc_proxy_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.cdc_proxy.email}"

  depends_on = [google_service_account.cdc_proxy]
}

# Container-Optimized OS runs the proxy from this declaration, so there is no shell script
resource "google_compute_instance_template" "cdc_proxy" {
  name_prefix  = "cdc-proxy-"
  machine_type = "e2-micro"
  tags         = ["cdc-proxy"]

  disk {
    source_image = "cos-cloud/cos-stable"
    auto_delete  = true
    boot         = true
  }

  network_interface {
    subnetwork = google_compute_subnetwork.subnet.id
  }

  service_account {
    email  = google_service_account.cdc_proxy.email
    scopes = ["cloud-platform"]
  }

  metadata = {
    google-logging-enabled = "true"

    gce-container-declaration = yamlencode({
      spec = {
        restartPolicy = "Always"
        containers = [{
          name  = "cloud-sql-proxy"
          image = var.cloud_sql_proxy_image
          args = [
            "--address=0.0.0.0",
            "--port=5432",
            "--private-ip",
            google_sql_database_instance.instance.connection_name,
          ]
          securityContext = { privileged = false }
          stdin           = false
          tty             = false
        }]
      }
    })
  }

  # The MIG references the template, so a new one has to exist before the old one goes
  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    google_project_iam_member.cdc_proxy_sql_client,
    google_compute_subnetwork.subnet,
    google_sql_database_instance.instance,
  ]
}

resource "google_compute_region_health_check" "proxy" {
  name   = "cdc-proxy-health"
  region = var.region

  timeout_sec         = 5
  check_interval_sec  = 10
  healthy_threshold   = 2
  unhealthy_threshold = 3

  tcp_health_check {
    port = 5432
  }

  depends_on = [google_project_service.required]
}

resource "google_compute_region_instance_group_manager" "cdc_proxy" {
  name                      = "cdc-proxy"
  region                    = var.region
  base_instance_name        = "cdc-proxy"
  distribution_policy_zones = var.proxy_zones
  target_size               = var.proxy_instance_count

  version {
    instance_template = google_compute_instance_template.cdc_proxy.id
  }

  named_port {
    name = "postgres"
    port = 5432
  }

  # Replaces a proxy that stops answering instead of leaving CDC dead
  auto_healing_policies {
    health_check      = google_compute_region_health_check.proxy.id
    initial_delay_sec = 180
  }

  # Native readiness gate: no gcloud, no provisioner
  wait_for_instances        = true
  wait_for_instances_status = "STABLE"

  depends_on = [
    google_compute_instance_template.cdc_proxy,
    google_compute_region_health_check.proxy,
    google_compute_firewall.allow_health_checks,
  ]
}

# Stable IP for Datastream: survives every proxy VM being replaced
resource "google_compute_address" "proxy_ilb" {
  name         = "cdc-proxy-ilb"
  region       = var.region
  subnetwork   = google_compute_subnetwork.subnet.id
  address_type = "INTERNAL"
  purpose      = "GCE_ENDPOINT"

  depends_on = [google_compute_subnetwork.subnet]
}

resource "google_compute_region_backend_service" "proxy" {
  name                  = "cdc-proxy-backend"
  region                = var.region
  protocol              = "TCP"
  load_balancing_scheme = "INTERNAL"
  health_checks         = [google_compute_region_health_check.proxy.id]

  # An INTERNAL backend service rejects UTILIZATION, the provider default
  backend {
    group          = google_compute_region_instance_group_manager.cdc_proxy.instance_group
    balancing_mode = "CONNECTION"
  }

  depends_on = [
    google_compute_region_instance_group_manager.cdc_proxy,
    google_compute_region_health_check.proxy,
  ]
}

resource "google_compute_forwarding_rule" "proxy" {
  name                  = "cdc-proxy-ilb"
  region                = var.region
  load_balancing_scheme = "INTERNAL"
  backend_service       = google_compute_region_backend_service.proxy.id
  ip_address            = google_compute_address.proxy_ilb.self_link
  ip_protocol           = "TCP"
  ports                 = ["5432"]
  subnetwork            = google_compute_subnetwork.subnet.id

  depends_on = [
    google_compute_region_backend_service.proxy,
    google_compute_address.proxy_ilb,
  ]
}
