variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "github_repo" {
  description = "GitHub repository in 'owner/repo' format (e.g. 'DreadPirateRobertz/teachers-lounge')"
  type        = string
}

variable "pool_id" {
  description = "Workload Identity Pool ID"
  type        = string
  default     = "github-actions"
}

variable "provider_id" {
  description = "Workload Identity Pool Provider ID"
  type        = string
  default     = "github-repo"
}

variable "service_account_id" {
  description = "Service account ID for GitHub Actions CI/CD"
  type        = string
  default     = "github-actions-cicd"
}
