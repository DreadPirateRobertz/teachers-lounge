# TeachersLounge

An AI-powered chatbot tutor that learns from a student's course materials and adapts to their individual learning style over time. Fully gamified with boss battles, XP progression, streaks, and a Neon Arcade visual identity.

> **No free tier. No ads.** Subscription-only, distraction-free learning environment.

---

## What It Does

Students upload textbooks, syllabi, quizzes, and lecture materials. The AI ingests them via RAG and becomes a patient, personalized tutor — **Professor Nova** — for that subject.

The platform is designed to be addictive: boss battles, XP, daily quests, leaderboards, and a persistent Student Knowledge Model (SKM) that grows smarter the longer you use it. The longer you use TeachersLounge, the more your tutor knows about how *you* think.

**Target users**: College/university students, professional/continuing education learners.

---

## Architecture

Full Kubernetes deployment on GKE. All services containerized, orchestrated via Helm charts.

```
Client (Next.js + WebGL)
        │
   GKE Gateway (Ingress + TLS)
        │
┌───────┼───────────────────────┐
│       │                       │
Frontend  Tutoring Service   Gaming Service
(Next.js) (Agentic RAG/SSE)  (XP/Bosses/Quests)
        │
┌───────┼───────────────────────┐
│       │           │           │
User    Search    Ingestion  Notification
Service Service   Service    Service
(Auth)  (Hybrid   (PDF/Video/ (Push/Email)
        Vector+KW) Audio)
        │
┌───────┼───────┬───────┬───────┐
Postgres  Qdrant  Redis  GCS  BigQuery
(Cloud SQL) (3-rep) (cache) (files) (analytics)
        │
   AI Gateway (LiteLLM)
```

---

## Services

| Service | Tech | Responsibility |
|---------|------|----------------|
| `frontend` | Next.js (App Router), Docker | SSR pages, Neon Arcade UI, Three.js boss battles |
| `user-service` | Go | Auth (JWT + OAuth), profiles, subscriptions, GDPR |
| `tutoring-service` | Python/FastAPI | Agentic RAG, Prof. Nova persona, SSE streaming |
| `ai-gateway` | LiteLLM on GKE | Unified LLM proxy — all services call this, never providers directly |
| `gaming-service` | Go | XP, levels, streaks, quests, leaderboards, boss state |
| `ingestion-service` | Python | PDF/doc/video/audio processing → Qdrant + Postgres |
| `search-service` | Python/FastAPI | Hybrid vector+keyword search, re-ranking |
| `notification-service` | Go/Node.js | Push (FCM), email (SendGrid/Resend), streak alerts |
| `analytics-service` | Python (CronJob) | Anonymize → aggregate → BigQuery, RAGAS eval |

---

## Data Stores

| Store | Purpose |
|-------|---------|
| **Cloud SQL (Postgres)** | Source of truth — users, auth, interactions, subscriptions, gaming profiles |
| **Qdrant** | Vector search — curriculum content (dense + sparse), concept embeddings, cross-student insights |
| **Redis (Memorystore)** | Real-time gaming state — leaderboards (sorted sets), boss battles, streaks, session cache |
| **GCS** | Raw file uploads, processed assets, data export bundles, Qdrant backups |
| **BigQuery** | Anonymized analytics — topic difficulty, explanation effectiveness, learning curves |

---

## Build Phases

| Phase | Weeks | Goal |
|-------|-------|------|
| **1 — Foundation** | 1-6 | Sign up, subscribe, chat with Prof. Nova |
| **2 — RAG Core** | 5-10 | Upload a PDF textbook, get grounded answers with page references |
| **3 — Gaming Layer** | 9-14 | XP, streaks, quests, leaderboard, AI-generated quizzes |
| **4 — Boss Battles** | 13-18 | Three.js animated boss fights at chapter completion |
| **5 — Adaptive Learning** | 17-22 | SKM, learning style detection, spaced repetition |
| **6 — Full Ingestion** | 21-26 | Video, audio, handwriting, Office docs, LaTeX |
| **7 — Analytics + Privacy** | 25-30 | FERPA/GDPR compliance, cross-student insights |
| **8 — Polish + Scale** | 29-34 | Notifications, mobile, performance, monitoring |

---

## Repo Layout

```
teachers-lounge/
├── services/
│   ├── frontend/          # Next.js app
│   ├── user-service/      # Go auth + subscriptions
│   ├── tutoring-service/  # Python agentic RAG
│   ├── ai-gateway/        # LiteLLM config + Helm
│   ├── gaming-service/    # Go XP/boss engine
│   ├── ingestion-service/ # Python file processing
│   ├── search-service/    # Python hybrid search
│   ├── notification-service/
│   └── analytics-service/
├── infra/
│   ├── helm/              # Helm charts per service
│   └── k8s/               # Base manifests
├── .github/workflows/     # CI/CD (build → Artifact Registry → ArgoCD)
└── docs/
    └── tv-tutor-design.md # Full design specification
```

---

## Infrastructure

- **Cluster**: GKE Autopilot (or Standard with node auto-provisioning)
- **Networking**: VPC-native, private cluster, Cloud NAT egress
- **Service mesh**: Istio or GKE Dataplane V2 (mTLS between services)
- **CI/CD**: GitHub Actions → Docker → Artifact Registry → ArgoCD (GitOps)
- **Monitoring**: Prometheus + Grafana (or Cloud Monitoring)
- **Logging**: Fluentbit → Cloud Logging

---

## Key Design Decisions

- **Self-hosted everything core**: LiteLLM gateway, Qdrant vector DB — no external platform lock-in for core functionality
- **SKM is the moat**: Student Knowledge Model grows over years; the longer you use it, the smarter your tutor becomes about *you*
- **Subscription-only**: No free tier, no ads — controlled, distraction-free environment
- **FERPA from day one**: Row-level security, audit trail, data export, right to erasure
- **K-12 ready (deferred)**: Schema supports minor account types and parental consent — not implemented at launch to avoid COPPA complexity

---

## Team (Ender's Game)

| Crew | Role |
|------|------|
| **petra** | PM — planning, coordination, bead lifecycle |
| **bean** | Backend/infra — GKE, Cloud SQL, Redis, auth |
| **alai** | AI — Tutoring Service, RAG pipeline, AI Gateway |
| **dink** | Data — Ingestion Service, Qdrant, Search Service |
| **shen** | Frontend — Next.js, Neon Arcade UI, Three.js |
| **carn** | Ops — Gaming Service, Notifications, DevOps/CI |

---

## Full Spec

See [docs/tv-tutor-design.md](docs/tv-tutor-design.md) for complete design: data models, RAG pipeline, gaming system, SKM, subscription logic, privacy/compliance, and UI/UX.
