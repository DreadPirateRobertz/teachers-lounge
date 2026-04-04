resource "google_artifact_registry_repository" "services" {
  provider = google-beta

  project       = var.project_id
  location      = var.region
  repository_id = "services"
  format        = "DOCKER"
  description   = "Docker images for TeachersLounge microservices"

  cleanup_policies {
    id     = "keep-recent"
    action = "KEEP"

    most_recent_versions {
      keep_count = 10
    }
  }

  cleanup_policies {
    id     = "delete-untagged"
    action = "DELETE"

    condition {
      tag_state = "UNTAGGED"
      older_than = "604800s" # 7 days
    }
  }
}

resource "google_project_iam_member" "gke_ar_reader" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${var.project_id}.svc.id.goog[default/default]"
}
