# GitHub Actions Workload Identity Federation
#
# Creates the GCP-side resources that allow GitHub Actions to authenticate
# as a GCP service account without storing long-lived credentials.
#
# Flow:
#   1. GitHub emits a short-lived OIDC token for each workflow run.
#   2. google-github-actions/auth exchanges it with the WIF provider.
#   3. GCP mints a short-lived access token scoped to the SA's permissions.
#
# After applying, pipe the two Terraform outputs into GitHub secrets:
#   scripts/setup-github-secrets.sh

# ── Workload Identity Pool ────────────────────────────────────────────────────

resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = var.pool_id
  display_name              = "GitHub Actions"
  description               = "Allows GitHub Actions workflows to authenticate via OIDC"
}

# ── Workload Identity Pool Provider (GitHub OIDC) ────────────────────────────

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = var.provider_id
  display_name                       = "GitHub OIDC"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  # Map GitHub JWT claims to Google attributes used in the binding below.
  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.actor"      = "assertion.actor"
    "attribute.repository" = "assertion.repository"
  }

  # Restrict to the single target repo so other forks cannot impersonate the SA.
  attribute_condition = "assertion.repository == '${var.github_repo}'"
}

# ── Service Account ───────────────────────────────────────────────────────────

resource "google_service_account" "github_actions" {
  project      = var.project_id
  account_id   = var.service_account_id
  display_name = "GitHub Actions CI/CD"
  description  = "Impersonated by GitHub Actions via Workload Identity Federation to push Docker images"
}

# ── Artifact Registry write permission ───────────────────────────────────────

resource "google_project_iam_member" "github_actions_ar_writer" {
  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# ── Allow WIF tokens from this repo to impersonate the SA ────────────────────

resource "google_service_account_iam_binding" "github_actions_wif" {
  service_account_id = google_service_account.github_actions.name
  role               = "roles/iam.workloadIdentityUser"

  members = [
    # Any workflow in the configured repo (all branches/PRs).
    "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repo}",
  ]
}
