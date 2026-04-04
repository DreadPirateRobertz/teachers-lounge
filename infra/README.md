# infra

Infrastructure for TeachersLounge on GKE.

## Structure

```
infra/
├── terraform/          # GCP infrastructure (VPC, GKE Autopilot, Cloud NAT, Artifact Registry)
│   ├── modules/        # Terraform modules
│   │   ├── vpc/        # VPC-native networking with secondary ranges
│   │   ├── gke/        # GKE Autopilot cluster (Dataplane V2 for mTLS)
│   │   ├── nat/        # Cloud NAT for private node egress
│   │   ├── artifact-registry/  # Docker image repository
│   │   └── monitoring/ # Cloud Monitoring alerts
│   └── env/            # Per-environment tfvars (dev, prod)
├── helm/               # Helm charts for each microservice
├── argocd/             # ArgoCD app-of-apps GitOps config
├── db/                 # Database migrations
├── k8s/                # Raw K8s manifests (if any)
└── redis/              # Redis configuration
```

## Terraform

```bash
cd infra/terraform
terraform init
terraform plan -var-file=env/prod.tfvars
terraform apply -var-file=env/prod.tfvars
```

Required GCP APIs: `container.googleapis.com`, `artifactregistry.googleapis.com`,
`compute.googleapis.com`, `monitoring.googleapis.com`.

### GitHub Actions Secrets

| Secret | Description |
|--------|-------------|
| `WIF_PROVIDER` | Workload Identity Federation provider resource name |
| `WIF_SERVICE_ACCOUNT` | GCP service account for CI/CD |

## ArgoCD

See [argocd/README.md](argocd/README.md) for bootstrap instructions.
