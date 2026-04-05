output "bucket_name" {
  description = "GCS bucket name for Qdrant snapshots."
  value       = google_storage_bucket.qdrant_snapshots.name
}

output "gcp_sa_email" {
  description = "GCP service account email to use as the Workload Identity annotation on the K8s ServiceAccount."
  value       = google_service_account.qdrant_snapshots.email
}
