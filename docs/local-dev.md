# Local Development Guide

Get the full TeachersLounge stack running locally in under 5 minutes.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Docker Desktop | ≥ 4.28 | https://docs.docker.com/get-docker/ |
| Docker Compose | ≥ 2.24 (bundled with Desktop) | — |
| Git | any | — |

That's it. No local Node, Go, or Python required.

---

## Quick Start

```bash
# 1. Clone the repo
git clone git@github.com:DreadPirateRobertz/teachers-lounge.git
cd teachers-lounge

# 2. Create your local env file
cp .env.example .env.local
# Edit .env.local — add your Anthropic/OpenAI API keys to enable AI features
# (everything else runs fine with the placeholder defaults)

# 3. Start the stack
docker compose --env-file .env.local up --build

# 4. Open the app
open http://localhost:3000
```

All 6 services start in dependency order. The first build takes 2-4 minutes
(images are cached after that). Subsequent starts are ~30 seconds.

---

## Services

| Service | Local URL | Description |
|---------|-----------|-------------|
| Frontend | http://localhost:3000 | Next.js UI |
| User Service | http://localhost:8080 | Auth, profiles, subscriptions (Go) |
| Tutoring Service | http://localhost:8000 | AI tutor / chat (Python/FastAPI) |
| AI Gateway | http://localhost:4000 | LiteLLM proxy to AI providers |
| Postgres | localhost:5432 | Relational DB |
| Redis | localhost:6379 | Cache / session store |

---

## Dev Mode (Hot Reload)

Add the `dev` override to mount source code and enable hot reload:

```bash
docker compose --env-file .env.local \
  -f docker-compose.yml \
  -f docker-compose.dev.yml \
  up --build
```

| Service | Hot Reload Mechanism |
|---------|---------------------|
| Frontend | Next.js dev server (HMR via WebSocket) |
| User Service | [Air](https://github.com/air-verse/air) — rebuilds on `.go` file change |
| Tutoring Service | uvicorn `--reload` — restarts on `.py` file change |
| AI Gateway | No hot reload (config change requires restart) |

Edit files in `services/<name>/` and changes apply immediately.

---

## Common Commands

```bash
# Start everything (detached)
docker compose --env-file .env.local up -d

# View logs for a specific service
docker compose logs -f tutoring-service

# Rebuild a single service after dependency changes
docker compose --env-file .env.local up --build tutoring-service

# Stop everything (keeps volumes)
docker compose down

# Stop and wipe all data (fresh start)
docker compose down -v

# Run a one-off command in a container
docker compose exec postgres psql -U tl_app -d teacherslounge
docker compose exec redis redis-cli -a localredispassword
```

---

## Environment Variables

All variables are in `.env.example`. Copy to `.env.local` and customise.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ANTHROPIC_API_KEY` | For AI | placeholder | Claude API key |
| `OPENAI_API_KEY` | For AI (fallback) | placeholder | OpenAI API key |
| `LITELLM_MASTER_KEY` | No | `localdevmasterkey` | Auth key for AI Gateway |
| `JWT_SECRET` | No | placeholder | JWT signing secret |
| `POSTGRES_PASSWORD` | No | `localdevpassword` | Postgres password |
| `REDIS_PASSWORD` | No | `localredispassword` | Redis password |
| `STRIPE_SECRET_KEY` | No | placeholder | Stripe test key |
| `STRIPE_WEBHOOK_SECRET` | No | placeholder | Stripe webhook secret |

**Never commit `.env.local`.** It is gitignored.

---

## Data Persistence

Postgres and Redis data persist in named Docker volumes across restarts:

```
postgres-data  →  /var/lib/postgresql/data
redis-data     →  /data
```

To reset to a clean state: `docker compose down -v`

---

## Troubleshooting

**Port already in use**
```bash
# Find what's using port 5432
lsof -i :5432
# Change the host port in docker-compose.yml if needed (e.g. "5433:5432")
```

**Service fails health check / won't start**
```bash
docker compose logs <service-name>
```

**AI features not working (tutoring returns errors)**
- Check that `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` is set in `.env.local`
- Verify AI Gateway is healthy: `curl http://localhost:4000/health/liveliness`

**Frontend can't reach backend (CORS / network errors)**
- Services communicate over Docker's internal network by service name
- From the host, use `localhost:<port>` — not the Docker service names
- The frontend's `NEXT_PUBLIC_API_URL` should point to `http://localhost:8080`

**Slow first build on Apple Silicon (M1/M2/M3)**
- Docker pulls multi-platform images; the first pull is slow on arm64
- Subsequent builds use the layer cache and are much faster
- If builds fail: `docker system prune` to clear stale cache

---

## Architecture (Local vs Production)

| Component | Local | Production |
|-----------|-------|------------|
| Database | Docker Postgres 16 | Cloud SQL Postgres 16 |
| Redis | Docker Redis 7 | Google Memorystore |
| Deployments | Docker Compose | GKE + ArgoCD (see `infra/argocd/`) |
| TLS | None (HTTP) | GKE Ingress + Managed Cert |
| Image Registry | Local Docker build | Artifact Registry |
