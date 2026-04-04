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
