#!/usr/bin/env bash
# setup-github-secrets.sh — Apply WIF Terraform module and set GitHub secrets
#
# Run once after `terraform init` to provision GCP Workload Identity Federation
# resources and wire the outputs into GitHub Actions repository secrets.
#
# Prerequisites:
#   - gcloud auth application-default login (with roles/editor or Owner on the project)
#   - terraform >= 1.5 in PATH
#   - gh (GitHub CLI) authenticated: gh auth login
#   - GITHUB_REPO env var or pass as first arg (default: DreadPirateRobertz/teachers-lounge)
#
# Usage:
#   cd infra/terraform
#   terraform init
#   ../../scripts/setup-github-secrets.sh [owner/repo]

set -euo pipefail

REPO="${1:-${GITHUB_REPO:-DreadPirateRobertz/teachers-lounge}}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TF_DIR="${SCRIPT_DIR}/../infra/terraform"

echo "==> Applying github_wif Terraform module for repo: ${REPO}"
cd "${TF_DIR}"

terraform apply \
  -target=module.github_wif \
  -var="github_repo=${REPO}" \
  -auto-approve

echo "==> Reading Terraform outputs"
WIF_PROVIDER="$(terraform output -raw wif_provider)"
WIF_SA="$(terraform output -raw wif_service_account)"

echo "  WIF_PROVIDER        = ${WIF_PROVIDER}"
echo "  WIF_SERVICE_ACCOUNT = ${WIF_SA}"

echo "==> Setting GitHub Actions secrets on ${REPO}"
gh secret set WIF_PROVIDER        --body "${WIF_PROVIDER}" --repo "${REPO}"
gh secret set WIF_SERVICE_ACCOUNT --body "${WIF_SA}"       --repo "${REPO}"

echo "==> Done. Build & Push Docker Images workflow will now authenticate via WIF."
echo "    Verify at: https://github.com/${REPO}/settings/secrets/actions"
