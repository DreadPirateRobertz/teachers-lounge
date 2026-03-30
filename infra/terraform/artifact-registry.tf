# ── Artifact Registry ────────────────────────────────────────────────────────
# Multi-region "us" repository matches the existing image references in Helm
# values: us-docker.pkg.dev/teachers-lounge/services/<image>
resource "google_artifact_registry_repository" "services" {
  location      = var.artifact_registry_region  # "us" = multi-region
  repository_id = var.artifact_registry_id      # "services"
  description   = "TeachersLounge microservice images"
  format        = "DOCKER"

  cleanup_policies {
    id     = "keep-recent-releases"
    action = "KEEP"
    most_recent_versions {
      keep_count = 10
    }
  }

  cleanup_policies {
    id     = "delete-untagged-after-30d"
    action = "DELETE"
    condition {
      tag_state  = "UNTAGGED"
      older_than = "2592000s"  # 30 days
    }
  }
}

# Allow GitHub Actions SA to push images (writer role already granted in gke.tf)
# This output is used by the CI workflow
output "artifact_registry_url" {
  description = "Base URL for Docker image pushes: docker tag <img> <url>/<name>:<tag>"
  value       = "${var.artifact_registry_region}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_id}"
}
