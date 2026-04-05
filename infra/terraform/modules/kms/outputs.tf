output "key_ring_id" {
  description = "Full resource ID of the KMS key ring."
  value       = google_kms_key_ring.main.id
}

output "cloudsql_key_id" {
  description = "Full resource ID of the Cloud SQL CMEK key. Pass to the Cloud SQL instance resource as disk_encryption_key."
  value       = google_kms_crypto_key.cloudsql.id
}

output "gcs_key_id" {
  description = "Full resource ID of the GCS CMEK key. Pass to google_storage_bucket resources as encryption.default_kms_key_name."
  value       = google_kms_crypto_key.gcs.id
}

output "app_key_id" {
  description = "Full resource ID of the application-layer key."
  value       = google_kms_crypto_key.app.id
}
