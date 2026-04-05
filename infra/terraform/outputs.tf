output "cluster_name" {
  description = "GKE cluster name"
  value       = module.gke.cluster_name
}

output "cluster_endpoint" {
  description = "GKE cluster endpoint"
  value       = module.gke.cluster_endpoint
  sensitive   = true
}

output "artifact_registry_url" {
  description = "Artifact Registry Docker repository URL"
  value       = module.artifact_registry.repository_url
}

output "vpc_network" {
  description = "VPC network name"
  value       = module.vpc.network_name
}

output "kubeconfig_command" {
  description = "Command to configure kubectl"
  value       = "gcloud container clusters get-credentials ${module.gke.cluster_name} --region ${var.region} --project ${var.project_id}"
}

output "wif_provider" {
  description = "WIF provider resource name — value for WIF_PROVIDER GitHub secret"
  value       = module.github_wif.wif_provider
}

output "wif_service_account" {
  description = "GitHub Actions service account email — value for WIF_SERVICE_ACCOUNT GitHub secret"
  value       = module.github_wif.wif_service_account
}
