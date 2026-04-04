#!/usr/bin/env bash
# Install ArgoCD onto the GKE cluster via Helm.
# Run once during initial cluster bootstrap.
#
# Prerequisites:
#   - kubectl configured against the target GKE cluster
#   - helm >= 3.12
#   - cert-manager installed (for TLS — see infra/cert-manager/README.md)
#   - A ClusterIssuer named "letsencrypt-prod" or Google-managed cert annotation

set -euo pipefail

NAMESPACE="argocd"
RELEASE="argocd"
CHART_VERSION="7.7.0"   # pin — bump deliberately
REPO_URL="https://argoproj.github.io/argo-helm"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Adding Argo Helm repo"
helm repo add argo "${REPO_URL}" --force-update
helm repo update

echo "==> Creating namespace ${NAMESPACE}"
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

echo "==> Installing/upgrading ArgoCD (chart ${CHART_VERSION})"
helm upgrade --install "${RELEASE}" argo/argo-cd \
  --namespace "${NAMESPACE}" \
  --version "${CHART_VERSION}" \
  --values "${SCRIPT_DIR}/values.yaml" \
  --wait \
  --timeout 5m

echo "==> Applying GKE Ingress for ArgoCD UI"
kubectl apply -f "${SCRIPT_DIR}/ingress.yaml"

echo "==> Bootstrapping app-of-apps (dev)"
kubectl apply -f "${SCRIPT_DIR}/apps/root-dev.yaml"

echo "==> Bootstrapping app-of-apps (prod)"
kubectl apply -f "${SCRIPT_DIR}/apps/root-prod.yaml"

echo ""
echo "==> Done. ArgoCD is deploying. Check status with:"
echo "    kubectl get applications -n argocd"
echo "    argocd app list  (after argocd CLI login)"
echo ""
echo "Initial admin password (change after first login):"
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath='{.data.password}' | base64 -d && echo ""
