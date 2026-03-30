output "cluster_name" {
  description = "GKE cluster name"
  value       = google_container_cluster.cluster.name
}

output "cluster_location" {
  description = "GKE cluster location (region)"
  value       = google_container_cluster.cluster.location
}

output "cluster_endpoint" {
  description = "GKE control plane endpoint"
  value       = google_container_cluster.cluster.endpoint
  sensitive   = true
}

output "workload_identity_pool_provider" {
  description = "Full resource name for GitHub Actions Workload Identity Federation"
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "github_actions_sa_email" {
  description = "Service account email for GitHub Actions — set as GH secret GCP_SA_EMAIL"
  value       = google_service_account.github_actions.email
}

output "configure_kubectl_command" {
  description = "Run this after apply to configure kubectl"
  value       = "gcloud container clusters get-credentials ${var.cluster_name} --region ${var.region} --project ${var.project_id}"
}
