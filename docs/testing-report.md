# TeachersLounge — Testing Report

> **Living document.** Updated by Petra (PM) after each PR batch or sprint.
> Last updated: 2026-04-12

---

## Summary

| Service | Tests | Coverage | CI Status | Last updated |
|---------|-------|----------|-----------|--------------|
| frontend (Next.js) | 709 | ≥80% patch | ✅ Green | 2026-04-12 |
| tutoring-service (Python) | passing | ≥80% patch | ✅ Green | 2026-04-12 |
| ingestion-service (Python) | passing | ≥80% patch | ✅ Green | 2026-04-12 |
| search-service (Python) | passing | ≥80% patch | ✅ Green | 2026-04-12 |
| analytics-service (Python) | passing | ≥80% patch | ✅ Green | 2026-04-12 |
| eval-service (Python) | passing | ≥80% patch | ✅ Green | 2026-04-12 |
| user-service (Go) | passing | ≥80% patch | ✅ Green | 2026-04-12 |
| gaming-service (Go) | passing | ≥80% patch | ✅ Green | 2026-04-12 |
| notification-service (Go) | passing | ≥80% patch | ✅ Green | 2026-04-12 |

---

## How to Run Tests Locally

### Frontend
```bash
cd frontend
npm test                      # jest watch mode
npm test -- --watchAll=false  # single run (CI mode)
npm run lint                  # eslint
npm run format:check          # prettier
```

### Python services (tutoring, ingestion, search, analytics, eval)
```bash
cd services/<service-name>
pip install -r requirements.txt
pytest                         # run all tests
pytest --cov=app --cov-report=term-missing   # with coverage
ruff check .                   # lint
```

### Go services (user-service, gaming-service, notification-service)
```bash
cd services/<service-name>
go test ./...                  # run all tests
go test ./... -coverprofile=coverage.out && go tool cover -summary=coverage.out
go vet ./...                   # static analysis
golangci-lint run              # lint
```

### Full stack (Docker Compose on Linux)
```bash
# On pop-os: ~/gt/teachers-lounge
cp .env.example .env.local     # fill in API keys
docker compose --env-file .env.local up --build
# Frontend: http://<machine-ip>:3000
```

---

## CI Gates (enforced on every PR)

| Gate | Threshold | Enforced by |
|------|-----------|-------------|
| Test coverage (patch) | ≥ 80% | Codecov |
| Frontend tests | all pass | GitHub Actions |
| Go tests | all pass | GitHub Actions |
| Python tests | all pass | GitHub Actions |
| Prettier format | clean | GitHub Actions |
| ESLint | clean | GitHub Actions |
| ruff | clean | GitHub Actions |
| go vet | clean | GitHub Actions |
| golangci-lint | clean | GitHub Actions |

Coverage gate was raised to **90% patch** as of PR #144 for new features.
Legacy code grandfathered at 80%.

---

## Open PRs — Test Status

| PR | Branch | Tests | Blocker |
|----|--------|-------|---------|
| [#167](../../pull/167) | feat/tl-d1u-onboarding-polish | ✅ 709 passing | None — ready to merge |
| [#206](../../pull/206) | feat/tl-8gw-csp-hardening | ✅ Tests pass | ❌ Prettier format check — run `npm run format` in `frontend/` |
| [#145](../../pull/145) | feat/hq-cv-eebuu-material-upload-ui | ✅ Tests pass | ❌ Prettier format check — run `npm run format` in `frontend/` |

---

## Docker Stack Status (pop-os dev machine)

| Container | Status | Port |
|-----------|--------|------|
| postgres | ✅ healthy | 127.0.0.1:5432 |
| redis | ✅ healthy | 127.0.0.1:6379 |
| ai-gateway (LiteLLM) | ✅ healthy | 127.0.0.1:4000 |
| user-service | ✅ healthy | 127.0.0.1:8080 |
| tutoring-service | ✅ healthy | 127.0.0.1:8000 |
| analytics-service | ✅ healthy | 127.0.0.1:8085 |
| frontend | ⚠️ unhealthy* | 0.0.0.0:3000 |
| gaming-service | 🔄 building | 127.0.0.1:8083 |
| ingestion-service | 🔄 building | 127.0.0.1:8001 |
| search-service | 🔄 building | 127.0.0.1:8002 |
| notification-service | 🔄 building | 127.0.0.1:9000 |
| qdrant | 🔄 starting | 127.0.0.1:6333 |

> *Frontend "unhealthy" is a false alarm — BusyBox wget resolves `localhost` as
> IPv6 (::1) but Next.js binds IPv4 only. Fixed in docker-compose.yml
> (127.0.0.1 explicit) — will clear on next restart. `curl localhost:3000/api/health`
> returns `{"status":"ok"}` from the host.

**Last stack pull:** 2026-04-12 from `main` @ `9b11693`

---

## Integration / Smoke Tests

These are manual smoke checks run against the docker stack. Automate with k6 (tl-3j2).

| Scenario | Status | Notes |
|----------|--------|-------|
| User registration + login | ⬜ pending | Run against localhost:3000 |
| Chat with tutoring service | ⬜ pending | Requires ANTHROPIC_API_KEY |
| File upload → ingestion | ⬜ pending | Requires GCS bucket config |
| Gaming: XP award | ⬜ pending | |
| Boss battle flow | ⬜ pending | |
| Flashcard creation + review | ⬜ pending | |
| Leaderboard display | ⬜ pending | |
| Subscription flow (Stripe test) | ⬜ pending | Requires Stripe test keys |

---

## Known Issues

| Issue | Severity | Status | Bead |
|-------|----------|--------|------|
| Frontend healthcheck false-negative (localhost vs 127.0.0.1) | Low | Fix in `main` — needs restart | — |
| gaming-service, ingestion, search, notification not in original docker-compose | Medium | Fixed `main` @ 9b11693 | — |
| PRs #145, #206 blocked on Prettier formatting | Low | Authors notified | — |

---

## Test Debt

| Area | Gap | Bead |
|------|-----|------|
| Integration / end-to-end tests | No automated E2E (Playwright/k6) yet | tl-3j2 |
| Smoke tests against docker stack | Manual only | tl-3j2 |
| Docstring coverage audit | Not yet audited against standard | tl-hf9 |

---

## Update Protocol

**PM (Petra)** updates this doc:
- After each PR merge batch
- After any docker stack change
- After any CI gate change
- After integration test runs

Format: edit table rows in place. Add dated notes for significant events.
