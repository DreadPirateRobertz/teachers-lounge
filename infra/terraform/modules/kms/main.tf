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

## Grant the Cloud SQL service account permission to use the cloudsql key.
## The service account identity follows the pattern
## service-<project_number>@gcp-sa-cloud-sql.iam.gserviceaccount.com.
## The project number must be provided via var.project_number.
resource "google_kms_crypto_key_iam_member" "cloudsql_sa" {
  crypto_key_id = google_kms_crypto_key.cloudsql.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:service-${var.project_number}@gcp-sa-cloud-sql.iam.gserviceaccount.com"
}

## Grant the GCS service account permission to use the gcs key.
## The service account identity follows the pattern
## service-<project_number>@gs-project-accounts.iam.gserviceaccount.com.
resource "google_kms_crypto_key_iam_member" "gcs_sa" {
  crypto_key_id = google_kms_crypto_key.gcs.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:service-${var.project_number}@gs-project-accounts.iam.gserviceaccount.com"
}
