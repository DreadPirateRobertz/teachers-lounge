# CI/Ops Audit — tl-96k
**Date:** 2026-04-05  
**Auditor:** carn (teachers_lounge)  
**Scope:** All merged PRs since Phase 1 kickoff (#3–#115)  
**Audit criteria:** CI compliance, godoc/docstring coverage, test coverage ≥80%, structured logging, WHY-only comments

---

## Summary

| Area | Status | Blockers |
|------|--------|---------|
| CI lint gates (Go + Python) | ❌ FAIL | Lint failures silently pass — `continue-on-error: true` / `\|\| true` |
| Python test gates | ❌ FAIL | Test failures silently pass — `\|\| echo "No tests yet"` |
| Go services — godoc | ✅ PASS | 0 missing across user-service, gaming-service, notification-service |
| Go services — tests | ✅ PASS | Comprehensive _test.go files in all packages |
| Go services — structured logging | ✅ PASS | slog (user-service), zap (gaming, notification) |
| Python services — tests | ✅ PASS | All have tests/ with pytest |
| Python services — docstrings | ⚠️ PARTIAL | tutoring: 38, ingestion: 9, search: 7, analytics: 1 missing |
| Python services — structured logging | ❌ FAIL | All 4 use `logging.basicConfig` (text) — no JSON output |
| Frontend — ESLint | ✅ PASS | Added PR #69; enforced in CI |
| Frontend — Prettier | ✅ PASS | Added PR #89; `format:check` in CI |
| Frontend — tests | ✅ PASS | Component tests PR #85, API route tests PR #117 |
| Frontend — JSDoc | ⚠️ PARTIAL | Inconsistent across components (see below) |
| Frontend — structured logging | ❌ FAIL | console.log/error throughout; no structured logger |

---

## Critical Blockers

### B1: CI lint gates not enforced (ci.yml:139, ci.yml:194)
**File:** `.github/workflows/ci.yml`

```yaml
# Line 139 — Go linting silently passes on errors:
continue-on-error: true

# Line 194 — Python ruff silently passes on errors:
- run: ruff check . || true
```

Both allow code with lint errors to merge. `golangci-lint` and `ruff` must be hard gates.  
**Child bead needed:** Fix CI lint enforcement.

### B2: Python test failures silently pass (ci.yml:198)
**File:** `.github/workflows/ci.yml`

```yaml
# Line 198 — pytest failures are swallowed:
- run: pytest --tb=short -q --cov=app --cov-report=xml:coverage.xml 2>/dev/null || echo "No tests yet"
```

`2>/dev/null` suppresses stderr and `|| echo "No tests yet"` means exit code 1 from a failing test never fails CI. This has been silently masking test regressions since Phase 1.  
**Child bead needed:** Fix Python test gate in CI.

### B3: Python services — unstructured logging
All 4 Python services use `logging.basicConfig` with text format. In Kubernetes, log aggregation (Loki/Stackdriver) requires JSON-structured logs.

| Service | File | Issue |
|---------|------|-------|
| tutoring-service | app/main.py:38 | `logging.basicConfig(level=...)` text format |
| analytics-service | app/main.py:18 | `logging.basicConfig(level=...)` text format |
| ingestion | app/main.py:15 | `logging.basicConfig(..., format="%(asctime)s ...")` text format |
| search | app/main.py:9 | `logging.basicConfig(..., format="%(asctime)s ...")` text format |

**Child bead needed:** Add JSON structured logging to all 4 Python services.

---

## Docstring Gaps (non-blocking, but must fix before next PR)

### tutoring-service — 38 missing docstrings
Key gaps:
- `app/main.py`: `on_startup`, `on_shutdown`, `health`, `readiness`
- `app/auth.py`: `JWTClaims` class
- Multiple service classes and methods in `app/services/`

### ingestion — 9 missing docstrings
- `app/main.py`: `lifespan`, `healthz`
- `app/config.py`: `Settings` class
- `app/models.py`: `ProcessingStatus`, `UploadResponse`

### search — 7 missing docstrings
- `app/main.py`: `lifespan`, `healthz`
- `app/config.py`: `Settings`
- `app/models.py`: `ChunkResult`, `SearchResponse`

### analytics-service — 1 missing docstring
- `app/database.py:15`: `get_db` async function

### frontend — JSDoc
`console.log`/`console.error` used throughout components; no structured logger. JSDoc missing on several components in `components/boss/`, `components/chat/`.

---

## Clean Areas (no action required)

- **Go services — godoc**: 0 missing exported funcs/types without godoc across user-service, gaming-service, notification-service (verified by AST scan).
- **Go services — structured logging**: slog (user-service), zap (gaming-service, notification-service). No bare `log.Printf`/`fmt.Printf` used for runtime logging.
- **Go services — tests**: All packages have `_test.go` files. user-service: 10 test files, gaming-service: 22 test files, notification-service: 8 test files.
- **Python services — tests**: tutoring-service (14 test files), search (4 test files), analytics-service (has tests/), ingestion (has tests/).
- **CI — change detection**: `dorny/paths-filter` correctly skips unchanged services.
- **CI — Docker builds**: All service Dockerfiles are built and verified on every docker path change.
- **CI — Helm lint**: All Helm charts linted on infra changes.
- **CI — Security**: TruffleHog secrets scan on every PR; `npm audit` on frontend.
- **CI — Codecov**: 90% project / 80% patch thresholds configured; coverage uploaded for all services.
- **Commit style**: Consistent `feat(scope):`/`fix(scope):` messages throughout.

---

## Child Beads to Open

| Bead | Title | Priority | Owner |
|------|-------|----------|-------|
| tl-96k-b1 | Fix CI lint gates — remove `continue-on-error` from golangci-lint, `\|\| true` from ruff | P1 | carn |
| tl-96k-b2 | Fix Python test gate — pytest must fail CI on test errors | P1 | carn |
| tl-96k-b3 | JSON structured logging for all 4 Python services | P2 | dink/alai |
| tl-96k-b4 | Docstrings sweep — tutoring-service (38), ingestion (9), search (7) | P2 | alai/dink |
