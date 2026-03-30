#!/usr/bin/env bash
# Bootstrap the TeachersLounge GKE cluster end-to-end.
#
# Prerequisites:
#   - gcloud authenticated: gcloud auth application-default login
#   - terraform >= 1.6 installed
#   - helm >= 3.12 installed
#   - kubectl installed
#   - GCP project "teachers-lounge" created and billing enabled
#   - APIs enabled (run step 0 below first)
#
# Usage:
#   ./infra/bootstrap.sh                 # Full bootstrap
#   ./infra/bootstrap.sh --tf-only       # Terraform only
#   ./infra/bootstrap.sh --k8s-only      # K8s setup only (cluster already exists)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ID="${PROJECT_ID:-teachers-lounge}"
REGION="${REGION:-us-central1}"
CLUSTER_NAME="${CLUSTER_NAME:-teachers-lounge-cluster}"

TF_ONLY="${1:-}"
K8S_ONLY="${1:-}"

log() { echo "==> $*"; }
die() { echo "ERROR: $*" >&2; exit 1; }

# ── Step 0: Enable required GCP APIs ─────────────────────────────────────────
enable_apis() {
  log "Enabling required GCP APIs..."
  gcloud services enable \
    container.googleapis.com \
    artifactregistry.googleapis.com \
    compute.googleapis.com \
    cloudresourcemanager.googleapis.com \
    iam.googleapis.com \
    iamcredentials.googleapis.com \
    sts.googleapis.com \
    logging.googleapis.com \
    monitoring.googleapis.com \
    --project "${PROJECT_ID}"
}

# ── Step 1: Create Terraform state bucket ────────────────────────────────────
create_tf_state_bucket() {
  BUCKET="teachers-lounge-tfstate"
  if ! gsutil ls -b "gs://${BUCKET}" &>/dev/null; then
    log "Creating Terraform state bucket: ${BUCKET}"
    gsutil mb -p "${PROJECT_ID}" -l "${REGION}" "gs://${BUCKET}"
    gsutil versioning set on "gs://${BUCKET}"
    gsutil lifecycle set "${SCRIPT_DIR}/terraform/tfstate-lifecycle.json" "gs://${BUCKET}" 2>/dev/null || true
  else
    log "Terraform state bucket already exists: ${BUCKET}"
  fi
}

# ── Step 2: Terraform apply ───────────────────────────────────────────────────
tf_apply() {
  log "Running Terraform..."
  cd "${SCRIPT_DIR}/terraform"
  terraform init -upgrade
  terraform validate
  terraform plan -out=tfplan
  terraform apply tfplan
  cd - > /dev/null
}

# ── Step 3: Configure kubectl ─────────────────────────────────────────────────
configure_kubectl() {
  log "Configuring kubectl for cluster ${CLUSTER_NAME}..."
  gcloud container clusters get-credentials "${CLUSTER_NAME}" \
    --region "${REGION}" \
    --project "${PROJECT_ID}"
  kubectl cluster-info
}

# ── Step 4: Install ArgoCD ────────────────────────────────────────────────────
install_argocd() {
  log "Installing ArgoCD..."
  "${SCRIPT_DIR}/argocd/install.sh"
}

# ── Step 5: Install monitoring stack ─────────────────────────────────────────
install_monitoring() {
  log "Installing kube-prometheus-stack..."
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
  helm repo update

  kubectl apply -f "${SCRIPT_DIR}/k8s/monitoring/namespace.yaml"

  # Create Grafana admin secret if it doesn't exist
  if ! kubectl get secret grafana-admin -n monitoring &>/dev/null; then
    log "Creating grafana-admin secret (you will be prompted for password)..."
    read -rsp "Grafana admin password: " GRAFANA_PASS
    echo
    kubectl create secret generic grafana-admin \
      --from-literal=admin-password="${GRAFANA_PASS}" \
      -n monitoring
  fi

  helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
    --namespace monitoring \
    --values "${SCRIPT_DIR}/k8s/monitoring/kube-prometheus-stack-values.yaml" \
    --wait --timeout 10m
}

# ── Step 6: Install Fluent Bit ────────────────────────────────────────────────
install_logging() {
  log "Installing Fluent Bit..."
  helm repo add fluent https://fluent.github.io/helm-charts --force-update
  helm repo update

  kubectl create namespace logging --dry-run=client -o yaml | kubectl apply -f -

  helm upgrade --install fluent-bit fluent/fluent-bit \
    --namespace logging \
    --values "${SCRIPT_DIR}/k8s/logging/fluentbit-values.yaml" \
    --wait --timeout 5m
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
  if [[ "${TF_ONLY}" == "--tf-only" ]]; then
    enable_apis
    create_tf_state_bucket
    tf_apply
    configure_kubectl
    exit 0
  fi

  if [[ "${K8S_ONLY}" == "--k8s-only" ]]; then
    configure_kubectl
    install_argocd
    install_monitoring
    install_logging
    exit 0
  fi

  # Full bootstrap
  enable_apis
  create_tf_state_bucket
  tf_apply
  configure_kubectl
  install_argocd
  install_monitoring
  install_logging

  log "Bootstrap complete."
  echo ""
  echo "Next steps:"
  echo "  1. Set GitHub secrets (from terraform output):"
  echo "     GCP_WORKLOAD_IDENTITY_PROVIDER"
  echo "     GCP_SA_EMAIL"
  echo "  2. Push to main — build-push.yml will build and push all service images"
  echo "  3. ArgoCD will sync apps from infra/argocd/apps/prod/"
  echo "  4. Grafana: https://grafana.teacherslounge.app (credentials in grafana-admin secret)"
  echo "  5. ArgoCD: https://argocd.teacherslounge.app"
}

main
