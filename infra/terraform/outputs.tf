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

output "qdrant_snapshot_bucket" {
  description = "GCS bucket name for Qdrant nightly snapshots"
  value       = module.qdrant_gcs.bucket_name
}

output "qdrant_snapshot_sa_email" {
  description = "GCP service account email for the Helm values-prod.yaml Workload Identity annotation"
  value       = module.qdrant_gcs.gcp_sa_email
}
