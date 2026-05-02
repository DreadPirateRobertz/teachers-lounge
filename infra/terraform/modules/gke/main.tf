resource "google_container_cluster" "primary" {
  provider = google-beta

  name     = var.cluster_name
  project  = var.project_id
  location = var.region

  # Autopilot mode — Google manages node pools, scaling, and upgrades
  enable_autopilot = true

  network    = var.network
  subnetwork = var.subnetwork

  ip_allocation_policy {
    cluster_secondary_range_name  = var.pods_range_name
    services_secondary_range_name = var.services_range_name
  }

  # Private cluster — nodes have no public IPs, egress via Cloud NAT
  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false # Allow kubectl from authorized networks
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }

  # GKE Dataplane V2 (eBPF-based) — provides mTLS, network policy, observability
  datapath_provider = "ADVANCED_DATAPATH"

  # Release channel for automatic upgrades
  release_channel {
    channel = "REGULAR"
  }

  # Workload Identity for secure pod-to-GCP-service auth
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # Binary Authorization (optional, can enable later)
  binary_authorization {
    evaluation_mode = "DISABLED"
  }

  # Logging and monitoring via Cloud Operations
  logging_config {
    enable_components = [
      "SYSTEM_COMPONENTS",
      "WORKLOADS",
    ]
  }

  monitoring_config {
    enable_components = [
      "SYSTEM_COMPONENTS",
    ]

    managed_prometheus {
      enabled = true
    }
  }

  # Maintenance window — weekdays 2-6 AM UTC
  maintenance_policy {
    recurring_window {
      start_time = "2024-01-01T02:00:00Z"
      end_time   = "2024-01-01T06:00:00Z"
      recurrence = "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR"
    }
  }

  deletion_protection = true
}
