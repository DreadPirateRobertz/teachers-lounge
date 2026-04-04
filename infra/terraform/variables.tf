variable "project_id" {
  description = "GCP project ID"
  type        = string
  default     = "teachers-lounge"
}

variable "region" {
  description = "GCP region for the cluster"
  type        = string
  default     = "us-central1"
}

variable "environment" {
  description = "Environment name (dev, prod)"
  type        = string
  default     = "prod"
}

variable "cluster_name" {
  description = "Name of the GKE cluster"
  type        = string
  default     = "tl-cluster"
}
