provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

module "vpc" {
  source = "./modules/vpc"

  project_id   = var.project_id
  region       = var.region
  cluster_name = var.cluster_name
}

module "nat" {
  source = "./modules/nat"

  project_id = var.project_id
  region     = var.region
  network    = module.vpc.network_name
  router     = "${var.cluster_name}-router"
}

module "gke" {
  source = "./modules/gke"

  project_id   = var.project_id
  region       = var.region
  cluster_name = var.cluster_name
  network      = module.vpc.network_name
  subnetwork   = module.vpc.subnet_name

  pods_range_name     = module.vpc.pods_range_name
  services_range_name = module.vpc.services_range_name

  depends_on = [module.vpc, module.nat]
}

module "artifact_registry" {
  source = "./modules/artifact-registry"

  project_id = var.project_id
  region     = "us"
}

module "monitoring" {
  source = "./modules/monitoring"

  project_id   = var.project_id
  cluster_name = var.cluster_name

  depends_on = [module.gke]
}

## CMEK encryption keys — Cloud SQL, GCS, and application-layer.
## Must be applied before Cloud SQL instances or GCS buckets are created so
## the service accounts can be granted CryptoKeyEncrypterDecrypter access.
module "kms" {
  source = "./modules/kms"

  project_id     = var.project_id
  project_number = var.project_number
  cluster_name   = var.cluster_name
  kms_location   = var.region
}

## Qdrant GCS snapshot bucket + Workload Identity IAM.
## The bucket name and GCP SA email are output for use in the Helm values-prod.yaml.
module "qdrant_gcs" {
  source = "./modules/qdrant-gcs"

  project_id  = var.project_id
  region      = var.region
  environment = var.environment
  bucket_name = "teachers-lounge-qdrant-snapshots-${var.environment}"

  ## Rotate out snapshots after 30 days in prod, 7 in dev.
  retention_days = var.environment == "prod" ? 30 : 7

  ## Must match the namespace and SA name used by the Helm release.
  k8s_namespace = "qdrant"
  k8s_sa_name   = "qdrant"

  depends_on = [module.gke]
}
