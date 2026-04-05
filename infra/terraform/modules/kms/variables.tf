variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "project_number" {
  description = "GCP project number (numeric). Used to construct managed service account emails for Cloud SQL and GCS CMEK grants."
  type        = string
}

variable "kms_location" {
  description = "Cloud KMS key ring location. Should match the region of the resources being encrypted (e.g. us-central1) or use a multi-region (us, global)."
  type        = string
  default     = "us-central1"
}

variable "cluster_name" {
  description = "GKE cluster name — used as a prefix in the key ring name to tie keys to the cluster."
  type        = string
}

variable "rotation_period" {
  description = "Automatic key rotation period in seconds. Defaults to 90 days (7776000s). Google rotates the key material; old versions remain for decryption."
  type        = string
  default     = "7776000s" # 90 days
}
