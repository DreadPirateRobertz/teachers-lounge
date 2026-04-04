resource "google_compute_router" "router" {
  name    = var.router
  project = var.project_id
  region  = var.region
  network = var.network
}

resource "google_compute_router_nat" "nat" {
  name                               = "${var.router}-nat"
  project                            = var.project_id
  region                             = var.region
  router                             = google_compute_router.router.name
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}
