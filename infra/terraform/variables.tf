variable "project_id" {
  description = "GCP project ID"
  type        = string
  default     = "teachers-lounge"
}

variable "region" {
  description = "GCP region for all resources"
  type        = string
  default     = "us-central1"
}

variable "cluster_name" {
  description = "GKE cluster name"
  type        = string
  default     = "teachers-lounge-cluster"
}

variable "network_name" {
  description = "VPC network name"
  type        = string
  default     = "teachers-lounge-vpc"
}

variable "subnet_name" {
  description = "Primary subnet name"
  type        = string
  default     = "teachers-lounge-subnet"
}

variable "subnet_cidr" {
  description = "Primary subnet CIDR"
  type        = string
  default     = "10.0.0.0/20"
}

variable "pods_cidr" {
  description = "Secondary range CIDR for GKE pods"
  type        = string
  default     = "10.1.0.0/16"
}

variable "services_cidr" {
  description = "Secondary range CIDR for GKE services"
  type        = string
  default     = "10.2.0.0/20"
}

variable "artifact_registry_region" {
  description = "Region for Artifact Registry (us = multi-region)"
  type        = string
  default     = "us"
}

variable "artifact_registry_id" {
  description = "Artifact Registry repository ID"
  type        = string
  default     = "services"
}
