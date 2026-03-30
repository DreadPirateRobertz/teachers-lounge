# ── Workload Identity bindings for cluster workloads ────────────────────────
# Each service account pair: GCP SA + Kubernetes SA bound via Workload Identity.
# This eliminates credential files stored as Kubernetes Secrets.

## Fluent Bit — log writer
resource "google_service_account" "fluentbit" {
  account_id   = "fluentbit"
  display_name = "Fluent Bit log shipper"
}

resource "google_project_iam_member" "fluentbit_log_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.fluentbit.email}"
}

resource "google_service_account_iam_member" "fluentbit_wif" {
  service_account_id = google_service_account.fluentbit.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[logging/fluent-bit]"
}

## ArgoCD — read Artifact Registry to pull chart and image metadata
resource "google_service_account" "argocd" {
  account_id   = "argocd"
  display_name = "ArgoCD GitOps controller"
}

resource "google_project_iam_member" "argocd_ar_reader" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.argocd.email}"
}

resource "google_service_account_iam_member" "argocd_wif" {
  service_account_id = google_service_account.argocd.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[argocd/argocd-server]"
}
