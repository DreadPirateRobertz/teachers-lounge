variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "region" {
  description = "GCS bucket location (region or multi-region, e.g. US, us-central1)."
  type        = string
  default     = "us-central1"
}

variable "environment" {
  description = "Environment label (dev, prod)."
  type        = string
}

variable "bucket_name" {
  description = "GCS bucket name for Qdrant snapshots. Must be globally unique."
  type        = string
}

variable "retention_days" {
  description = "Number of days after which snapshots are automatically deleted."
  type        = number
  default     = 30
}

variable "k8s_namespace" {
  description = "Kubernetes namespace where the Qdrant StatefulSet is deployed."
  type        = string
  default     = "qdrant"
}

variable "k8s_sa_name" {
  description = "Name of the Kubernetes ServiceAccount used by the snapshot CronJob."
  type        = string
  default     = "qdrant"
}
