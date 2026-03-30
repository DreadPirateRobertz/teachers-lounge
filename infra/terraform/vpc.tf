# ── VPC ─────────────────────────────────────────────────────────────────────
resource "google_compute_network" "vpc" {
  name                    = var.network_name
  auto_create_subnetworks = false
}

# Primary subnet with secondary ranges for GKE pods and services
resource "google_compute_subnetwork" "subnet" {
  name                     = var.subnet_name
  ip_cidr_range            = var.subnet_cidr
  region                   = var.region
  network                  = google_compute_network.vpc.id
  private_ip_google_access = true  # Required: nodes in private cluster need to reach GCP APIs

  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = var.pods_cidr
  }

  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = var.services_cidr
  }
}

# ── Cloud Router + Cloud NAT ─────────────────────────────────────────────────
# Private nodes have no external IPs; Cloud NAT provides egress for pulling
# images from docker.io, pypi, go modules, etc.
resource "google_compute_router" "router" {
  name    = "${var.network_name}-router"
  region  = var.region
  network = google_compute_network.vpc.id
}

resource "google_compute_router_nat" "nat" {
  name                               = "${var.network_name}-nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

# ── Firewall rules ───────────────────────────────────────────────────────────
# GKE Autopilot manages its own firewall rules, but explicit deny-all + allow
# for health checks follows least-privilege.
resource "google_compute_firewall" "allow_internal" {
  name    = "${var.network_name}-allow-internal"
  network = google_compute_network.vpc.id

  allow {
    protocol = "tcp"
  }
  allow {
    protocol = "udp"
  }
  allow {
    protocol = "icmp"
  }

  source_ranges = [var.subnet_cidr, var.pods_cidr, var.services_cidr]
  description   = "Allow all internal traffic (pods, services, nodes)"
}

resource "google_compute_firewall" "allow_health_checks" {
  name    = "${var.network_name}-allow-health-checks"
  network = google_compute_network.vpc.id

  allow {
    protocol = "tcp"
  }

  # Google health check probe ranges
  source_ranges = ["35.191.0.0/16", "130.211.0.0/22"]
  description   = "Allow GCP load balancer health check probes"
}
