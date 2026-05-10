output "workload_identity_provider" {
  description = "The full identifier of the Workload Identity Provider"
  value       = google_iam_workload_identity_pool_provider.github_provider.name
}

output "service_account_email" {
  description = "The email of the GitHub Actions service account"
  value       = google_service_account.github_actions.email
}
