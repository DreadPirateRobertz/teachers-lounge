## CMEK (Customer-Managed Encryption Keys) for TeachersLounge.
##
## Provisions a Cloud KMS key ring with three crypto keys:
##
##   cloudsql   — encrypts Cloud SQL database disk (CMEK on the Cloud SQL instance)
##   gcs        — encrypts GCS buckets (snapshot storage, Qdrant backups)
##   app        — generic application-level key for secrets or field-level encryption
##
## Each key rotates automatically every rotation_period (default 90 days).
## All three keys are in the same key ring for operational simplicity; they can be
## split into separate rings per key if policy requires different IAM boundaries.
##
## After applying, grant Cloud SQL and GCS service accounts the
## roles/cloudkms.cryptoKeyEncrypterDecrypter role on the respective keys so they
## can use CMEK at rest.

resource "google_kms_key_ring" "main" {
  name     = "${var.cluster_name}-keyring"
  location = var.kms_location
  project  = var.project_id
}

## Cloud SQL CMEK key — used to encrypt database disks.
resource "google_kms_crypto_key" "cloudsql" {
  name            = "cloudsql"
  key_ring        = google_kms_key_ring.main.id
  rotation_period = var.rotation_period

  lifecycle {
    ## Crypto keys cannot be deleted via Terraform once data has been encrypted.
    ## Prevent accidents; keys must be destroyed manually through GCP console.
    prevent_destroy = true
  }
}

## GCS CMEK key — used to encrypt object storage (Qdrant snapshots, exports).
resource "google_kms_crypto_key" "gcs" {
  name            = "gcs"
  key_ring        = google_kms_key_ring.main.id
  rotation_period = var.rotation_period

  lifecycle {
    prevent_destroy = true
  }
}

## Application key — available for field-level encryption of PII or secrets.
## Not currently wired to any GCP service; available for application-layer use.
resource "google_kms_crypto_key" "app" {
  name            = "app"
  key_ring        = google_kms_key_ring.main.id
  rotation_period = var.rotation_period

  lifecycle {
    prevent_destroy = true
  }
}

## Ensure the Cloud SQL service agent exists before granting permissions.
resource "google_project_service_identity" "cloudsql" {
  provider = google-beta
  project  = var.project_id
  service  = "sqladmin.googleapis.com"
}

## Grant the Cloud SQL service account permission to use the cloudsql key.
resource "google_kms_crypto_key_iam_member" "cloudsql_sa" {
  crypto_key_id = google_kms_crypto_key.cloudsql.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_project_service_identity.cloudsql.email}"
}

## Ensure the GCS service agent exists before granting permissions.
data "google_storage_project_service_account" "gcs" {
  project = var.project_id
}

## Grant the GCS service account permission to use the gcs key.
resource "google_kms_crypto_key_iam_member" "gcs_sa" {
  crypto_key_id = google_kms_crypto_key.gcs.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${data.google_storage_project_service_account.gcs.email_address}"
}
