# AI Gateway — LiteLLM on GKE

Internal proxy for all LLM provider calls. **Never call provider APIs directly from services** — always route through this gateway.

## Phase 1 Configuration

- **Provider**: Anthropic
- **Models**: `tutor-primary` (claude-sonnet-4-6) · `tutor-fast` (claude-haiku-4-5)
- **Internal URL**: `http://ai-gateway.teachers-lounge.svc.cluster.local:4000`
- **Auth**: `LITELLM_MASTER_KEY` (shared with all services via K8s Secret)

## Deployment (Helm)

```bash
# Add upstream chart dependency
helm repo add litellm https://helm.litellm.ai
helm dependency update ./helm

# Populate secrets first (or use External Secrets Operator)
kubectl create secret generic litellm-secrets -n teachers-lounge \
  --from-literal=ANTHROPIC_API_KEY=<key> \
  --from-literal=LITELLM_MASTER_KEY=<key> \
  --from-literal=LITELLM_DATABASE_URL=postgresql://...

# Deploy (dev)
helm upgrade --install ai-gateway ./helm \
  -f helm/values.yaml \
  -n teachers-lounge --create-namespace

# Deploy (prod)
helm upgrade --install ai-gateway ./helm \
  -f helm/values.yaml \
  -f helm/values.prod.yaml \
  -n teachers-lounge
```

## Rate Limits (Phase 1)

| Tier       | RPM | TPM     |
|------------|-----|---------|
| default    | 60  | 100,000 |
| premium    | 300 | 500,000 |

Set per-user limits via LiteLLM API on subscription creation (Phase 3).

## Adding Providers (Phase 2+)

Add entries to `litellm_config.yaml` → `model_list`. The model alias stays the same — services don't need to change.
