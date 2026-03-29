# ArgoCD / GitOps Setup

All deployments to the TeachersLounge GKE cluster go through ArgoCD. Manual
`kubectl apply` is replaced by git commits. If it isn't in git, it doesn't run
in the cluster.

---

## Architecture

```
infra/argocd/apps/
├── root-dev.yaml        ← Root Application — watches apps/dev/
├── root-prod.yaml       ← Root Application — watches apps/prod/
├── dev/
│   ├── frontend.yaml
│   ├── user-service.yaml
│   ├── tutoring-service.yaml
│   ├── ai-gateway.yaml
│   ├── postgres.yaml    ← dev only (in-cluster Postgres)
│   └── redis.yaml       ← dev only (in-cluster Redis)
└── prod/
    ├── frontend.yaml
    ├── user-service.yaml
    ├── tutoring-service.yaml
    ├── ai-gateway.yaml
    ├── postgres.yaml    ← placeholder (Cloud SQL in prod)
    └── redis.yaml       ← placeholder (Memorystore in prod)

infra/helm/
├── frontend/
├── user-service/
├── tutoring-service/
├── ai-gateway/
├── postgres/
└── redis/
```

Each service has three value files:
- `values.yaml` — base defaults
- `values-dev.yaml` — dev overrides (lower replicas, debug logging, in-cluster DBs)
- `values-prod.yaml` — prod overrides (higher replicas, Cloud SQL / Memorystore)

The app-of-apps pattern means ArgoCD manages its own child Applications. To add
a new service: add a new Application YAML to `apps/dev/` and `apps/prod/`, and a
Helm chart to `infra/helm/<service>/`. Push to main — ArgoCD picks it up within
~3 minutes.

---

## Initial Bootstrap

### Prerequisites

| Tool | Version |
|------|---------|
| kubectl | ≥ 1.29, configured for the GKE cluster |
| helm | ≥ 3.12 |
| gcloud | logged in, project set to `teachers-lounge` |

### 1. Reserve a static IP for the ArgoCD UI

```bash
gcloud compute addresses create argocd-ip --global
gcloud compute addresses describe argocd-ip --global --format="value(address)"
```

### 2. Point DNS to that IP

Add an A record: `argocd.teacherslounge.app → <IP from above>`

Wait for DNS to propagate before continuing (required for TLS provisioning).

### 3. Run the install script

```bash
cd infra/argocd
./install.sh
```

This:
1. Installs ArgoCD via Helm into the `argocd` namespace
2. Applies the GKE Ingress + ManagedCertificate for TLS
3. Applies the two root Applications (`root-dev`, `root-prod`)

ArgoCD then syncs all child Applications automatically. The first sync pulls
the Helm charts from this repo and deploys all Phase 1 services.

### 4. Log into the ArgoCD UI

```
https://argocd.teacherslounge.app
Username: admin
Password: (printed at the end of install.sh, also: kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d)
```

**Change the admin password immediately after first login.**

### 5. Create required Secrets

Each service reads credentials from a Kubernetes Secret. Create these before
ArgoCD syncs (or services will start but fail on secret lookup):

```bash
# Postgres (dev namespace)
kubectl create secret generic postgres-secrets -n dev \
  --from-literal=postgres-password='<generate-strong-password>'

# User Service
kubectl create secret generic user-service-secrets -n dev \
  --from-literal=db-password='<same-as-postgres-password>' \
  --from-literal=jwt-secret='<generate-256-bit-random>' \
  --from-literal=stripe-secret-key='sk_test_...' \
  --from-literal=stripe-webhook-secret='whsec_...'

# Tutoring Service
kubectl create secret generic tutoring-service-secrets -n dev \
  --from-literal=db-password='<same-as-postgres-password>' \
  --from-literal=ai-gateway-api-key='<litellm-master-key>'

# AI Gateway
kubectl create secret generic ai-gateway-secrets -n dev \
  --from-literal=openai-api-key='sk-...' \
  --from-literal=anthropic-api-key='sk-ant-...' \
  --from-literal=litellm-master-key='<generate-random>'

# Redis (dev namespace)
kubectl create secret generic redis-secrets -n dev \
  --from-literal=redis-password='<generate-strong-password>'
```

Repeat for the `prod` namespace with production values.

> **Note:** Long-term, migrate these to [Google Secret Manager with Workload Identity](https://cloud.google.com/secret-manager/docs/accessing-the-api) and the External Secrets Operator. Manual kubectl creates are acceptable for Phase 1.

---

## Day-to-Day Workflows

### Deploy a new image version

```bash
# Update the image tag in the relevant values-dev.yaml or values-prod.yaml
# Then commit and push:
git add infra/helm/frontend/values-prod.yaml
git commit -m "deploy: frontend v1.2.3 to prod"
git push
```

ArgoCD detects the change within ~3 minutes and rolls out the new image.

### Add a new service

1. Create `infra/helm/<service>/` with Chart.yaml, values.yaml, values-dev.yaml, values-prod.yaml, and templates/
2. Create `infra/argocd/apps/dev/<service>.yaml` and `infra/argocd/apps/prod/<service>.yaml`
3. Commit and push — ArgoCD creates the Application and deploys

### Force a manual sync (infra/PM roles only)

```bash
argocd app sync frontend-dev
# or via the UI: https://argocd.teacherslounge.app
```

### Roll back a deployment

```bash
argocd app history frontend-prod        # List previous syncs
argocd app rollback frontend-prod <ID>  # Roll back to a specific sync
```

To make the rollback permanent, revert the commit in git.

### Check sync status

```bash
kubectl get applications -n argocd
argocd app list
```

---

## RBAC

| Role | Capabilities |
|------|-------------|
| `infra` | Full access (sync, delete, exec, manage repos) |
| `pm` | View all, trigger manual sync, view logs |
| `readonly` (default) | View all — no write operations |

RBAC is configured in `values.yaml` under `configs.rbac`. To assign a role,
add the user/group to the `argocd-rbac-cm` ConfigMap or via the ArgoCD UI.

OIDC (Google Workspace) integration is stubbed in `values.yaml` — uncomment
and fill in the `oidc.config` block to enable SSO.

---

## Environments

| Env | Namespace | Cluster | Database |
|-----|-----------|---------|----------|
| dev | `dev` | GKE (same cluster as argocd) | In-cluster Postgres 16 + Redis 7 |
| prod | `prod` | GKE (same cluster) | Cloud SQL Postgres 16 + Memorystore Redis 7 |

The `postgres` and `redis` Helm charts are deployed to dev only. The prod
Applications for those services exist in ArgoCD as placeholders but deploy
no actual resources (the charts are no-ops when targeting prod).

---

## Troubleshooting

**ArgoCD stuck in "Progressing"**
```bash
kubectl describe application <app-name> -n argocd
argocd app get <app-name> --refresh
```

**Pod fails to start (ImagePullBackOff)**
- Ensure Artifact Registry permissions are set for the GKE node service account:
  `roles/artifactregistry.reader`

**Secret not found**
- Check that the Secret exists in the correct namespace:
  `kubectl get secrets -n dev`

**TLS certificate not provisioning**
- Verify DNS is pointing to the static IP before the ManagedCertificate was applied
- Check status: `kubectl describe managedcertificate argocd-tls -n argocd`
