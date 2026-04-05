output "wif_provider" {
  description = "Full resource name of the WIF provider — set as WIF_PROVIDER GitHub secret"
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "wif_service_account" {
  description = "Service account email — set as WIF_SERVICE_ACCOUNT GitHub secret"
  value       = google_service_account.github_actions.email
}

output "pool_name" {
  description = "Full resource name of the Workload Identity Pool"
  value       = google_iam_workload_identity_pool.github.name
}
