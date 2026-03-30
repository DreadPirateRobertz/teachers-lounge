terraform {
  required_version = ">= 1.6"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }

  # Remote state — bucket must be pre-created (bootstrap step)
  backend "gcs" {
    bucket = "teachers-lounge-tfstate"
    prefix = "gke/cluster"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
