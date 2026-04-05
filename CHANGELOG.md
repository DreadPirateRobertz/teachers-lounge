# Teachers Lounge — Changelog

> Updated by Petra (PM) after each merge batch. Links to every PR for traceability.

---

## 2026-04-05 — Sprint 1 Complete

### Phase 1: Foundation
| PR | What |
|----|------|
| [#71](../../pull/71) | fix(ci): user-service workflow — Go 1.25, golangci-lint version fix |
| [#76](../../pull/76) | feat(ci): Codecov integration — 90% coverage threshold, TDD enforced |
| [#77](../../pull/77) | fix(codecov): disable email notifications |
| [#89](../../pull/89) | feat(tl-4co): prettier --check in frontend CI |
| [#88](../../pull/88) | feat(tl-nei): JWT aud claim — set on issuance, validated by all services |
| [#102](../../pull/102) | fix: resolve duplicate 006 migration sequence |
| [#110](../../pull/110) | fix: resolve main branch compilation errors (IncrWithTTL, AuditAction, TickRivals) |
| [#111](../../pull/111) | chore(tl-a9n): CONTRIBUTING.md — TDD, logging, docstring standards |
| [#113](../../pull/113) | fix(docker): resolve 3 Docker build failures — cmake, Go 1.25, npm |
| [#118](../../pull/118) | chore(tl-96k): CI/ops audit — enforce lint + test gates |
| [#139](../../pull/139) | fix(ci): switch Docker CI to GHCR — free, no GCP billing needed |
| [#144](../../pull/144) | chore: enforce 90% patch coverage + expanded PR template |

### Phase 2: RAG Core
| PR | What |
|----|------|
| [#79](../../pull/79) | feat(tl-dkm): agentic RAG pipeline with hybrid search |
| [#93](../../pull/93) | feat(tl-vki): prerequisite-aware tutoring — gap detection in RAG pipeline |
| [#108](../../pull/108) | feat(tl-rly): Phase 6 full ingestion — office docs, video/audio, image OCR |
| [#109](../../pull/109) | feat(tl-d4h): multi-modal RAG — CLIP diagrams, KaTeX LaTeX, Three.js molecules |
| [#115](../../pull/115) | fix(ingestion): promote lazy imports to module level for testability |
| [#121](../../pull/121) | feat(tl-6k1+tl-dkg): hybrid search filters, BM25 sparse indexing |
| [#125](../../pull/125) | fix(tl-gkr): ingestion docker-build — pin onnx>=1.16.0 |
| [#128](../../pull/128) | feat(tl-4zc): ingestion Dockerfile multi-stage build, python:3.12-slim |
| [#130](../../pull/130) | feat(hq-cv-gjapg): ingestion JWT auth — aud-validated HS256 |
| [#131](../../pull/131) | feat(tl-okc): ruff.toml — docstring, import-order, complexity rules |
| [#132](../../pull/132) | feat(hq-cv-iguk6): ingestion service — material status endpoint |
| [#135](../../pull/135) | feat(hq-cv-iguk6): wire figure GCS upload — PDF processor uploads extracted images |
| [#129](../../pull/129) | feat(hq-cv-kqszc): Qdrant GKE production readiness — PDB, Helm test, GCS terraform |
| [#138](../../pull/138) | feat(hq-cv-gjapg): tutoring-service JWT audience validation + full docstrings |
| [#141](../../pull/141) | feat(tl-5ca): JSON structured logging — tutoring, ingestion, search, analytics |

### Phase 3: Gaming Layer
| PR | What |
|----|------|
| [#81](../../pull/81) | feat(tl-27h): simulated rivals — leaderboard NPCs with daily XP ticks |
| [#90](../../pull/90) | feat(tl-7r6): Redis token-bucket rate limiting on XP and quiz endpoints |
| [#92](../../pull/92) | feat(tl-27h): simulated rivals — leaderboard NPCs (expanded) |
| [#74](../../pull/74) | feat(tl-dbw): quest board UI — dedicated page, real API integration |
| [#142](../../pull/142) | feat(tl-a1n): power-up gem shop — backend API + frontend UI |

### Phase 4: Boss Battles
| PR | What |
|----|------|
| [#94](../../pull/94) | feat(tl-n9h): boss taunts — AI-generated via LiteLLM Claude Haiku |
| [#97](../../pull/97) | feat(tl-nu8): Weird Science battle effects — particles, shake, morph, VFX |

### Phase 5: Adaptive Learning
| PR | What |
|----|------|
| [#106](../../pull/106) | feat(tl-1e9): flashcard system — SM-2, Anki export, in-app viewer |
| [#120](../../pull/120) | feat(tl-vw6): SKM review queue — migration 014 + HTTP test suite |
| [#122](../../pull/122) | feat(tl-vw6): Student Knowledge Model — learning profiles, misconceptions, SRS |
| [#126](../../pull/126) | test(tl-vw6): boost tutoring-service patch coverage above 75% |

### Phase 7: Analytics + Privacy
| PR | What |
|----|------|
| [#72](../../pull/72) | feat(tl-a5v): analytics dashboard |
| [#78](../../pull/78) | feat(tl-nsg): pytest suite + docstrings for analytics-service |
| [#104](../../pull/104) | feat(tl-cgj): FERPA audit trail + GDPR compliance |

### Phase 8: Polish + Scale
| PR | What |
|----|------|
| [#91](../../pull/91) | feat(tl-z7o): notification push mechanics — FCM delivery, token registration |
| [#103](../../pull/103) | feat(tl-0x8): SendGrid email, event triggers, 3/day push rate limit |
| [#136](../../pull/136) | feat(tl-zti): security hardening — CMEK/KMS, Redis TLS, CSP headers, rate limiting |
| [#140](../../pull/140) | feat(tl-ixk): CSP nonce migration — remove `unsafe-inline` from `script-src` |

### Frontend + UX
| PR | What |
|----|------|
| [#75](../../pull/75) | feat(tl-udx): learning style assessment |
| [#80](../../pull/80) | feat(tl-udx): learning style detection — behavioral signals, Felder-Silverman |
| [#85](../../pull/85) | test(tl-fwg): frontend component tests — 73 tests, 7 components |
| [#100](../../pull/100) | feat(tl-pcw): sci-fi quote database — 225 quotes, Redis daily dedup |
| [#116](../../pull/116) | feat(tl-xgk): React error boundaries — boss WebGL, quests, flashcards |
| [#117](../../pull/117) | feat(tl-fwg): Jest tests for all frontend API routes |
| [#119](../../pull/119) | feat(tl-c5m): multi-modal responses — TTS audio player + notes panel |
| [#123](../../pull/123) | fix(tl-cqg): cookie security, auth 401s, makeGetRequest null-check |
| [#133](../../pull/133) | feat(tl-kfc): frontend coverage — 16 new test files, 573 total tests |
| [#143](../../pull/143) | feat(tl-num): onboarding flow — first-run wizard, character creation, upload guide |
| [#147](../../pull/147) | feat(tl-buv): mobile responsiveness + K-12 hooks |

### Docker / Local Dev
| PR | What |
|----|------|
| [#149](../../pull/149) | fix(docker): add DATABASE_URL to user-service |
| [#150](../../pull/150) | fix(docker): add missing Stripe price ID env vars to user-service |
| [#151](../../pull/151) | fix(docker): add REDIS_ADDR to user-service |
| [#152](../../pull/152) | fix(user-service): fix empty chi route pattern for DELETE /users/{id} |
| [#153](../../pull/153) | fix(docker): user-service healthcheck endpoint is /healthz not /health |
| [#154](../../pull/154) | fix(user-service): switch runtime image to alpine for healthcheck |
| [#155](../../pull/155) | fix(docker): add missing env vars to tutoring-service |
| [#156](../../pull/156) | fix(docker): add db-migrate init service for schema migrations |
| [#157](../../pull/157) | fix(docker): use pgvector/pgvector:pg16 for postgres image |
| [#158](../../pull/158) | fix(docker): tutoring-service port mapping 8000:8080 + healthcheck |
| [#159](../../pull/159) | fix(docker): bind frontend to 0.0.0.0 for LAN access |
| [#160](../../pull/160) | fix(frontend): disable HSTS for local dev — browser was upgrading to HTTPS |

### Fixes
| PR | What |
|----|------|
| [#86](../../pull/86) | fix(tl-ck9): subscription integration tests |
| [#137](../../pull/137) | fix: upload route tests — use valid UUIDs after security hardening |

---

*Last updated: 2026-04-05 by carn (tl-fos)*
