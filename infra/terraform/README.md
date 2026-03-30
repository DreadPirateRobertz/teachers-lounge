# Terraform — GKE Cluster Infrastructure

Provisions the full GKE cluster environment for TeachersLounge.

## What this creates

| Resource | Details |
|----------|---------|
| VPC | `teachers-lounge-vpc` — custom-mode, no auto-subnets |
| Subnet | `teachers-lounge-subnet` — `10.0.0.0/20`, pods `10.1.0.0/16`, services `10.2.0.0/20` |
| Cloud Router + NAT | Egress for private nodes (pull from docker.io, pypi, etc.) |
| GKE Autopilot cluster | `teachers-lounge-cluster` — regional (`us-central1`), VPC-native, private nodes |
| Artifact Registry | `us-docker.pkg.dev/teachers-lounge/services` — multi-region Docker repo |
| Workload Identity Pool | GitHub Actions OIDC pool for keyless CI authentication |
| Service accounts | `github-actions-ci`, `fluentbit`, `argocd` — each bound via Workload Identity |

## Prerequisites

```bash
# Install tools
brew install terraform google-cloud-sdk

# Authenticate
gcloud auth application-default login

# Enable APIs and bootstrap state bucket (first time only)
./infra/bootstrap.sh --tf-only
```

## Usage

```bash
cd infra/terraform

# First run — initialize backend (state bucket must exist)
terraform init

terraform plan
terraform apply
```

## After apply — set GitHub secrets

```bash
# Get values from Terraform outputs
terraform output workload_identity_pool_provider
terraform output github_actions_sa_email
```

Set in GitHub → Settings → Secrets → Actions:
- `GCP_WORKLOAD_IDENTITY_PROVIDER` — output from above
- `GCP_SA_EMAIL` — output from above

## Networking

- Nodes are **private** (no external IPs). Cloud NAT provides egress.
- Control plane is **public** with authorized networks (lock down to VPN CIDR in production).
- **GKE Dataplane V2** (eBPF) provides network policy enforcement and encrypted
  pod-to-pod traffic within the cluster — no Istio sidecar overhead required.

## Cluster access

```bash
# Configure kubectl after apply
$(terraform output -raw configure_kubectl_command)

# Verify
kubectl get nodes
kubectl get namespaces
```

## State

Remote state is stored in `gs://teachers-lounge-tfstate/gke/cluster`.
The bucket must be created before `terraform init` (the bootstrap script does this).
