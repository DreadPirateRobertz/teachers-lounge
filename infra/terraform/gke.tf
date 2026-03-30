# ── GKE Autopilot cluster ────────────────────────────────────────────────────
# Autopilot is preferred: no node pool management, automatic security hardening,
# and lower operational burden for a small team.
resource "google_container_cluster" "cluster" {
  name     = var.cluster_name
  location = var.region  # Regional cluster = HA control plane

  # Autopilot mode: node provisioning, scaling, and upgrades are fully managed
  enable_autopilot = true

  # VPC-native private cluster
  network    = google_compute_network.vpc.id
  subnetwork = google_compute_subnetwork.subnet.id

  networking_mode = "VPC_NATIVE"

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false  # Public endpoint with authorized networks
    master_ipv4_cidr_block  = "172.16.0.32/28"
  }

  # Restrict control plane access to known CIDR blocks
  # In production, lock this down to your office/VPN IPs or bastion
  master_authorized_networks_config {
    cidr_blocks {
      cidr_block   = "0.0.0.0/0"
      display_name = "all (restrict in production)"
    }
  }

  # GKE Dataplane V2 (eBPF-based) provides built-in mTLS-like network policies
  # and replaces the need for a full Istio sidecar mesh for intra-cluster encryption.
  datapath_provider = "ADVANCED_DATAPATH"

  # Workload Identity: pods authenticate to GCP APIs as service accounts
  # (no long-lived keys stored as Kubernetes Secrets)
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # Managed Prometheus: scrapes workloads automatically, stores in Google Managed Prometheus
  monitoring_config {
    enable_components = [
      "SYSTEM_COMPONENTS",
      "WORKLOADS",
    ]
    managed_prometheus {
      enabled = true
    }
  }

  logging_config {
    enable_components = [
      "SYSTEM_COMPONENTS",
      "WORKLOADS",
    ]
  }

  # Automatic binary authorization and security posture
  security_posture_config {
    mode               = "BASIC"
    vulnerability_mode = "VULNERABILITY_BASIC"
  }

  # Lifecycle: prevent accidental cluster deletion
  deletion_protection = true
}

# ── Workload Identity service account for CI/CD ──────────────────────────────
# GitHub Actions impersonates this SA via Workload Identity Federation
# (no long-lived JSON keys)
resource "google_service_account" "github_actions" {
  account_id   = "github-actions-ci"
  display_name = "GitHub Actions CI/CD"
}

resource "google_project_iam_member" "github_actions_ar_writer" {
  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_project_iam_member" "github_actions_gke_developer" {
  project = var.project_id
  role    = "roles/container.developer"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# Workload Identity Federation pool + provider for GitHub Actions
resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github-pool"
  display_name              = "GitHub Actions Pool"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-provider"
  display_name                       = "GitHub OIDC Provider"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_mapping = {
    "google.subject"             = "assertion.sub"
    "attribute.actor"            = "assertion.actor"
    "attribute.repository"       = "assertion.repository"
    "attribute.repository_owner" = "assertion.repository_owner"
  }

  # Only trust tokens from the DreadPirateRobertz org
  attribute_condition = "assertion.repository_owner == 'DreadPirateRobertz'"
}

resource "google_service_account_iam_member" "github_wif_binding" {
  service_account_id = google_service_account.github_actions.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository_owner/DreadPirateRobertz"
}
