variable "project_id" {
  type = string
}

variable "region" {
  description = "Artifact Registry location (e.g. us, us-central1)"
  type        = string
  default     = "us"
}
