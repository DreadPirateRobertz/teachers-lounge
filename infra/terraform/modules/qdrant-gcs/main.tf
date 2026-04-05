## Qdrant GCS snapshot infrastructure.
##
## Provisions:
##   1. GCS bucket for nightly Qdrant snapshots (lifecycle: delete after retention_days)
##   2. GCP service account for snapshot uploads (roles/storage.objectCreator)
##   3. Workload Identity binding: K8s SA "qdrant" in var.k8s_namespace → GCP SA
##
## The Helm chart's CronJob uses Workload Identity to authenticate — no static
## credentials required. After applying this module, annotate the K8s ServiceAccount
## with the GCP SA email (values-prod.yaml already has the annotation).

resource "google_storage_bucket" "qdrant_snapshots" {
  name          = var.bucket_name
  project       = var.project_id
  location      = var.region
  force_destroy = false

  ## Uniform bucket-level access — no per-object ACLs.
  uniform_bucket_level_access = true

  ## Versioning disabled — snapshots are full exports, not incremental.
  versioning {
    enabled = false
  }

  ## Delete snapshots older than retention_days to control storage costs.
  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      age = var.retention_days
    }
  }

  labels = {
    env     = var.environment
    managed = "terraform"
    app     = "qdrant"
  }
}

## GCP service account used by the Qdrant snapshot CronJob.
resource "google_service_account" "qdrant_snapshots" {
  account_id   = "qdrant-snapshots"
  display_name = "Qdrant Snapshot Uploader"
  description  = "Used by the Qdrant nightly snapshot CronJob via Workload Identity."
  project      = var.project_id
}

## Grant the service account permission to write objects to the snapshot bucket.
resource "google_storage_bucket_iam_member" "qdrant_snapshot_writer" {
  bucket = google_storage_bucket.qdrant_snapshots.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${google_service_account.qdrant_snapshots.email}"
}

## Workload Identity binding: the K8s ServiceAccount "qdrant" (in k8s_namespace)
## is allowed to impersonate the GCP SA, giving pods that mount it GCS access
## without static credentials.
resource "google_service_account_iam_member" "workload_identity_binding" {
  service_account_id = google_service_account.qdrant_snapshots.name
  role               = "roles/iam.workloadIdentityUser"
  ## Format: serviceAccount:<project>.svc.id.goog[<namespace>/<k8s-sa-name>]
  member = "serviceAccount:${var.project_id}.svc.id.goog[${var.k8s_namespace}/${var.k8s_sa_name}]"
}
