# CI Bypass Audit — tl-s7z
**Date:** 2026-04-05
**Auditor:** shen (teachers_lounge)
**Scope:** PRs #149–#160, all admin-merged without CI gating
**Status:** Complete

---

## Summary

All 12 PRs were merged by DreadPirateRobertz (repo owner) within a 36-minute
window (22:17–22:53 UTC on 2026-04-05) with no code reviews and branch
protection bypassed. The pattern is consistent with a live debugging session
standing up the local Docker Compose dev environment.

| Metric | Count |
|--------|-------|
| Total PRs audited | 12 |
| Admin-merged (no review) | 12 |
| Config-only (docker-compose.yml) | 3 |
| Code changes (Go/TS + config) | 9 |
| CI green at merge time | 11 |
| CI failing at merge time | 1 (PR #160) |
| CI failures on main post-merge | 3 (PRs #152, #154, #160) |

---

## Risk Assessment

### HIGH: PR #160 merged with failing CI
**PR:** `fix(frontend): disable HSTS for local dev — browser was upgrading to HTTPS`
**SHA:** `7e701f3`
**Issue:** `frontend` CI check was FAILING on the PR branch at merge time.
The failure persists on main post-merge. `build-push (frontend)` also fails.
**Files:** `docker-compose.yml`, `frontend/lib/csp.ts`, `frontend/next.config.ts`
**Impact:** Frontend CI broken on main until a subsequent fix lands.

### MEDIUM: PRs #152, #154 — build-push failures on main
- **PR #152** (`ddc0d65`): `build-push (user-service): failure` on main.
  Tests/lint passed. Code change: chi route pattern fix in `main.go`.
- **PR #154** (`5e49c22`): `build-push (user-service): failure` on main,
  several docker-build jobs cancelled. Code change: alpine image switch
  in Dockerfile + healthcheck in `main.go`.

### LOW: PR #158 — CI cancelled on main
**SHA:** `716966c` — most checks cancelled on main (superseded by next push).
Not a true failure, just incomplete CI data.

---

## Per-PR Detail

### PR #149 — `fix(docker): add DATABASE_URL to user-service`
- **SHA:** `1629ea3` | **Merged:** 22:17 UTC
- **CI at merge:** All SUCCESS (service checks skipped — config-only paths)
- **CI on main:** All green
- **Type:** Config-only (`docker-compose.yml`)

### PR #150 — `fix(docker): add missing Stripe price ID env vars to user-service`
- **SHA:** `8cba113` | **Merged:** 22:19 UTC
- **CI at merge:** All SUCCESS (service checks skipped)
- **CI on main:** All green
- **Type:** Config-only (`docker-compose.yml`)

### PR #151 — `fix(docker): add REDIS_ADDR to user-service`
- **SHA:** `04b4ad5` | **Merged:** 22:20 UTC
- **CI at merge:** All SUCCESS (service checks skipped)
- **CI on main:** All green
- **Type:** Config-only (`docker-compose.yml`)

### PR #152 — `fix(user-service): fix empty chi route pattern for DELETE /users/{id}`
- **SHA:** `ddc0d65` | **Merged:** 22:21 UTC
- **CI at merge:** All SUCCESS (go-services, integration tests, migrations passed)
- **CI on main:** PARTIAL FAILURE — `build-push (user-service): failure`
- **Type:** Code (`docker-compose.yml`, `services/user-service/cmd/server/main.go`)

### PR #153 — `fix(docker): user-service healthcheck endpoint is /healthz not /health`
- **SHA:** `b61b109` | **Merged:** 22:24 UTC
- **CI at merge:** All SUCCESS
- **CI on main:** All green
- **Type:** Code (`docker-compose.yml`, `services/user-service/cmd/server/main.go`)

### PR #154 — `fix(user-service): switch runtime image to alpine for healthcheck`
- **SHA:** `5e49c22` | **Merged:** 22:27 UTC
- **CI at merge:** All SUCCESS (docker-build suite passed)
- **CI on main:** PARTIAL FAILURE — `build-push (user-service): failure`, docker-build cancelled
- **Type:** Code (`docker-compose.yml`, `Dockerfile`, `main.go`)

### PR #155 — `fix(docker): add missing env vars to tutoring-service`
- **SHA:** `748e798` | **Merged:** 22:29 UTC
- **CI at merge:** Mostly SUCCESS; `docker-build (ingestion): CANCELLED`
- **CI on main:** All green
- **Type:** Code (`docker-compose.yml`, `Dockerfile`, `main.go`)

### PR #156 — `fix(docker): add db-migrate init service for schema migrations`
- **SHA:** `1d9fa62` | **Merged:** 22:32 UTC
- **CI at merge:** All SUCCESS (docker-build suite passed)
- **CI on main:** All green
- **Type:** Code (`docker-compose.yml`, `Dockerfile`, `main.go`)

### PR #157 — `fix(docker): use pgvector/pgvector:pg16 for postgres image`
- **SHA:** `20402f6` | **Merged:** 22:32 UTC
- **CI at merge:** All SUCCESS
- **CI on main:** All green
- **Type:** Code (`docker-compose.yml`, `Dockerfile`, `main.go`)

### PR #158 — `fix(docker): tutoring-service port mapping 8000:8080 + healthcheck`
- **SHA:** `716966c` | **Merged:** 22:38 UTC
- **CI at merge:** Mostly SUCCESS; `docker-build (ingestion): CANCELLED`
- **CI on main:** Mostly CANCELLED (superseded by next commit). CodeQL passed.
- **Type:** Code (`docker-compose.yml`, `Dockerfile`, `main.go`)

### PR #159 — `fix(docker): bind frontend to 0.0.0.0 for LAN access`
- **SHA:** `6599fda` | **Merged:** 22:49 UTC
- **CI at merge:** All SUCCESS
- **CI on main:** All green
- **Type:** Code (`docker-compose.yml`, `Dockerfile`, `main.go`)

### PR #160 — `fix(frontend): disable HSTS for local dev — browser was upgrading to HTTPS`
- **SHA:** `7e701f3` | **Merged:** 22:53 UTC
- **CI at merge:** FAILURE — `frontend` check failed
- **CI on main:** FAILURE — `frontend` + `build-push (frontend)` both failing
- **Type:** Code (`docker-compose.yml`, `frontend/lib/csp.ts`, `frontend/next.config.ts`)

---

## Recommendations

1. **Fix frontend CI on main** — PR #160 left main with a broken frontend build.
   This should be the immediate priority before further merges.
2. **Investigate build-push failures** — PRs #152 and #154 triggered
   `build-push (user-service)` failures that may indicate Docker image build issues.
3. **Require CI pass for admin merges** — even during debugging sessions, at
   minimum lint + test checks should pass. Consider removing admin bypass or
   adding a post-merge CI notification when bypass is used.
4. **Batch docker-compose fixes** — 12 PRs in 36 minutes for incremental
   `docker-compose.yml` tweaks could have been 2–3 PRs. Recommend batching
   related config changes to reduce merge noise and CI churn.
