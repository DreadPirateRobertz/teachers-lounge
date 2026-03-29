# TeachersLounge — Design Specification

**Date**: 2026-03-29
**Status**: Draft — Pending User Review
**Author**: Brainstorming session with project founder

---

## Table of Contents

1. [Product Vision](#1-product-vision)
2. [Target Users & Constraints](#2-target-users--constraints)
3. [System Architecture](#3-system-architecture)
4. [Microservice Breakdown](#4-microservice-breakdown)
5. [Data Model](#5-data-model)
6. [RAG Pipeline Design](#6-rag-pipeline-design)
7. [AI Gateway & Model Strategy](#7-ai-gateway--model-strategy)
8. [Gaming System Design](#8-gaming-system-design)
9. [Adaptive Learning System](#9-adaptive-learning-system)
10. [Student Knowledge Model (SKM)](#10-student-knowledge-model-skm)
11. [Subscription & Monetization](#11-subscription--monetization)
12. [Data Privacy & Compliance](#12-data-privacy--compliance)
13. [UI/UX Design — Neon Arcade](#13-uiux-design--neon-arcade)
14. [Phased Build Plan](#14-phased-build-plan)
15. [Open Design Decisions](#15-open-design-decisions)
16. [Research Appendices](#16-research-appendices)

---

## 1. Product Vision

TeachersLounge is an AI-powered chatbot tutor that learns from a student's course materials and adapts to their individual learning style over time. Students upload textbooks, syllabi, quizzes, and other materials. The AI ingests them via RAG and becomes a patient, personalized tutor for that subject.

The platform is designed to be **addictive** — a fully gamified learning experience with boss battles, XP progression, streaks, leaderboards, power-ups, and a Neon Arcade visual identity (blue/green/pink rave palette). The AI tutor is a named character (Professor Nova) with personality, and students create their own avatar with a class and rank system.

**Core differentiator**: The AI tutor grows with the student over years. The Student Knowledge Model (SKM) preserves the tutor's understanding of how each student thinks, what analogies work, where they struggle, and their full educational journey. This creates a switching cost moat — the longer you use TeachersLounge, the smarter your tutor becomes.

**No free tier. No ads.** A controlled, distraction-free learning environment. Subscription-only with a trial period.

---

## 2. Target Users & Constraints

### Launch target
- **College/university students** — self-directed learners, textbook-heavy courses
- **Professional/continuing education** — certifications, corporate training

### Future expansion
- **K-12 (middle/high school)** — architecture designed to support this but deferred to avoid COPPA complexity at launch. Skeleton hooks for parental controls, age-gated content, and guardian consent flows will be built into the User Service from the start.

### Key constraints
- Small, highly focused technical team (2-5 engineers)
- Full K8s on GKE — team wants full infrastructure control
- No external platform dependencies for core functionality (self-hosted AI gateway, self-hosted vector DB)
- FERPA compliance required from day one
- GDPR-ready for international expansion

---

## 3. System Architecture

### Overview

Full Kubernetes deployment on Google Kubernetes Engine (GKE). All services containerized (Docker), orchestrated via Helm charts. No serverless platform dependencies.

```
                        ┌──────────────┐
                        │   Client     │
                        │  (Browser)   │
                        │  Next.js App │
                        │  WebGL/Three │
                        └──────┬───────┘
                               │ HTTPS / WSS
                        ┌──────▼───────┐
                        │ GKE Gateway  │
                        │ (Ingress)    │
                        │ TLS + WAF    │
                        └──────┬───────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
     ┌────────▼──────┐ ┌──────▼──────┐ ┌───────▼──────┐
     │   Frontend    │ │  Tutoring   │ │   Gaming     │
     │   Service     │ │  Service    │ │   Service    │
     │  (Next.js)    │ │ (Agentic    │ │  (XP/Bosses  │
     │               │ │   RAG)      │ │   /Quests)   │
     └───────────────┘ └──────┬──────┘ └──────────────┘
                              │
     ┌────────────────────────┼────────────────────────┐
     │                        │                        │
┌────▼─────┐  ┌───────▼───────┐  ┌──────▼──────┐  ┌───▼───────┐
│  User    │  │    Search     │  │  Ingestion  │  │ Notif.    │
│ Service  │  │   Service     │  │  Service    │  │ Service   │
│ (Auth/   │  │  (Hybrid      │  │ (PDF/Img/   │  │ (Push/    │
│  Subs)   │  │   Vector+KW)  │  │  A-V proc)  │  │  Email)   │
└──────────┘  └───────────────┘  └─────────────┘  └───────────┘
                              │
     ┌────────────────────────┼────────────────────────┐
     │            │           │          │             │
┌────▼────┐ ┌────▼────┐ ┌────▼───┐ ┌────▼───┐ ┌──────▼──────┐
│Postgres │ │ Qdrant  │ │ Redis  │ │  GCS   │ │  BigQuery   │
│+pgvector│ │(3 repl) │ │(cache) │ │(files) │ │ (analytics) │
└─────────┘ └─────────┘ └────────┘ └────────┘ └─────────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
     ┌────────▼──────┐ ┌─────▼──────┐ ┌──────▼──────┐
     │  AI Gateway   │ │ Analytics  │ │  Neo4j /    │
     │  (LiteLLM /   │ │  Service   │ │  Postgres   │
     │   Portkey)    │ │ (CronJob)  │ │  ltree      │
     └───────────────┘ └────────────┘ └─────────────┘

External Services (dashed boundary):
  - Stripe (billing)
  - Google Document AI (OCR + handwriting)
  - Google Speech-to-Text / Whisper (audio/video transcription)
  - Cloud KMS (encryption key management)
  - SendGrid / Resend (transactional email)
  - Firebase Cloud Messaging (push notifications)
```

### GKE Cluster Configuration

- **Cluster type**: GKE Autopilot (reduces ops burden for small team) or Standard with node auto-provisioning
- **Node pools**:
  - `general-pool`: e2-standard-4 (4 vCPU, 16GB) — most services
  - `gpu-pool`: g2-standard-4 with NVIDIA L4 — embedding model inference (Phase 2+), optional
- **Networking**: VPC-native, private cluster with Cloud NAT for egress
- **Service mesh**: Istio or GKE Dataplane V2 for mTLS between services
- **CI/CD**: GitHub Actions → build Docker images → push to Artifact Registry → ArgoCD or Flux for GitOps deployments
- **Monitoring**: Prometheus + Grafana (self-hosted) or Google Cloud Monitoring
- **Logging**: Fluentbit → Cloud Logging or ELK stack

---

## 4. Microservice Breakdown

### 4.1 Frontend Service

- **Tech**: Next.js (App Router) in Docker
- **Responsibilities**: SSR pages, static asset serving, client-side WebGL/Three.js boss battle rendering
- **Endpoints**: Serves all routes, proxies API calls to backend services via GKE internal DNS
- **Replicas**: 2-3 behind Ingress
- **Key libraries**: React, Three.js (boss battles), Cannon.js or Rapier (physics), shadcn/ui (base components), TailwindCSS, KaTeX (math rendering)

### 4.2 Tutoring Service

- **Tech**: Python (FastAPI) or Go
- **Responsibilities**: Core AI brain. Receives student questions, orchestrates multi-step agentic RAG retrieval, generates personalized responses, streams via SSE
- **Key flow**: Question → SKM lookup → prerequisite check → curriculum retrieval → cross-student insights → LLM generation → stream response → update SKM → check gaming triggers
- **Replicas**: 2-4 (scales with concurrent chat sessions)
- **Dependencies**: Search Service, User Service (SKM), Gaming Service, AI Gateway
- **Latency target**: first token in <2 seconds

### 4.3 Gaming Service

- **Tech**: Go (performance-critical) or Node.js
- **Responsibilities**: XP calculation, level progression, streak management, boss battle state machine, quest tracking, achievement unlocks, leaderboard management, power-up inventory, sci-fi quote serving
- **Data store**: Redis (real-time gaming state, leaderboards via sorted sets) + Postgres (persistent progression data, achievement history)
- **Replicas**: 2-3
- **Key APIs**:
  - `POST /gaming/xp` — award XP for an action
  - `GET /gaming/profile/{userId}` — full gaming state (level, XP, streaks, badges)
  - `POST /gaming/boss/start` — initiate boss battle for a topic
  - `POST /gaming/boss/answer` — submit answer in boss fight, return damage/result
  - `GET /gaming/leaderboard` — top N + student's rank
  - `GET /gaming/quests/daily` — current daily quests and progress
  - `GET /gaming/quotes/random` — context-appropriate sci-fi quote

### 4.4 User Service

- **Tech**: Go or Node.js
- **Responsibilities**: Authentication (JWT + refresh tokens), user profile CRUD, subscription management (Stripe integration), learning preference storage, data export, account deletion
- **Auth flow**: Email/password + OAuth (Google, Apple). JWT access tokens (15 min) + refresh tokens (30 days) stored in HTTP-only cookies
- **Stripe integration**: Webhook receiver for subscription events (created, renewed, cancelled, payment_failed). Plans: monthly, quarterly, semesterly. Trial period configurable.
- **K-12 hooks**: Account type field (standard, minor). Parental consent flow skeleton. Age gate at registration. Not implemented at launch but schema supports it.
- **Replicas**: 2
- **Key APIs**:
  - `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh`
  - `GET /users/{id}/profile`, `PATCH /users/{id}/preferences`
  - `POST /users/{id}/export` — triggers async data export job
  - `DELETE /users/{id}` — GDPR right to erasure (cascading delete across all services)
  - `POST /webhooks/stripe` — subscription lifecycle

### 4.5 Ingestion Service

- **Tech**: Python (heavy use of unstructured.io, Document AI SDK, ffmpeg)
- **Responsibilities**: Accepts file uploads, routes through type-specific processing pipelines, chunks documents, generates embeddings, writes to Qdrant and Postgres
- **Processing pipeline**:
  1. File received → stored in GCS (raw)
  2. Type detection → route to processor
  3. **PDF (digital)**: unstructured.io → layout-aware parsing → hierarchical chunking
  4. **PDF (scanned)/Images**: Google Document AI → OCR → chunking
  5. **Handwriting**: Document AI handwriting model → text extraction
  6. **Office docs**: python-docx/pptx/openpyxl → HTML → chunking
  7. **Video**: ffmpeg (extract audio) → Whisper or Speech-to-Text → transcript → chunking
  8. **Audio**: Whisper or Speech-to-Text → transcript → chunking
  9. **LaTeX**: Pandoc/custom parser → preserve equation structure
  10. Chunks → metadata tagging (chapter, section, page, content type, course ID)
  11. Chunks → embedding model (e5-large-v2 self-hosted or OpenAI API)
  12. Embeddings → Qdrant (curriculum collection)
  13. Metadata → Postgres (chunk registry)
  14. LLM-assisted concept extraction → knowledge graph (Neo4j/Postgres)
- **Deployment**: GKE Jobs for heavy processing (not long-running pods). Pub/Sub triggers.
- **Chunking strategy**:
  - Hierarchical: respect document structure (chapters → sections → paragraphs)
  - Max chunk size: 512-1024 tokens
  - Never split mid-sentence, mid-equation, or mid-table
  - Context enrichment: prepend hierarchical path to each chunk ("Chapter 3 > Section 3.2 > Quadratic Formula")
  - Special handling: figures (store image in GCS, text chunk with caption), tables (Markdown), equations (LaTeX string), quiz questions (structured JSON)

### 4.6 Search Service

- **Tech**: Python (FastAPI) or Go
- **Responsibilities**: Wraps Qdrant with hybrid search, knowledge graph traversal, result re-ranking
- **Search modes**:
  - **Hybrid** (default): dense vector (semantic) + sparse vector (BM25 keyword) via Qdrant native support. Reciprocal Rank Fusion for combining results.
  - **Filtered**: search within a specific course, chapter, or content type using Qdrant payload filters
  - **Graph-augmented**: traverse knowledge graph for related concepts, expand search to prerequisite/related topics
  - **Multi-modal** (Phase 6): CLIP-based image similarity search for diagram retrieval
- **Re-ranking**: LLM-based re-ranker or cross-encoder model to improve top-k relevance
- **Replicas**: 2-3
- **Latency target**: <200ms for top-10 results

### 4.7 Analytics Service

- **Tech**: Python
- **Deployment**: GKE CronJob (nightly)
- **Responsibilities**:
  1. Query Postgres for all interactions from past 24 hours
  2. Strip PII (replace student IDs with anonymous session hashes)
  3. Aggregate by topic: error rates, common wrong answers, time-to-correct, explanation effectiveness
  4. Write to BigQuery
  5. Generate aggregated insight embeddings → Qdrant (insights collection) or Postgres cache
  6. These insights feed back into the Tutoring Service's prompts ("72% of students struggle with step 3 here")
- **Anonymization**: k-anonymity (minimum group size of 10), differential privacy for published reports
- **Evaluation**: RAGAS framework (weekly) for RAG quality metrics (faithfulness, relevance, correctness). Custom "learning effectiveness" metric: correlation between tutoring interactions and subsequent quiz performance.

### 4.8 Notification Service

- **Tech**: Node.js or Go
- **Responsibilities**: Push notifications (Firebase Cloud Messaging), email (SendGrid/Resend), in-app notifications
- **Trigger types**:
  - Streak-at-risk warnings ("Study today to keep your 14-day streak!")
  - Weak spot alerts ("Stereochem at 42% — 3x XP if you practice now")
  - Rival notifications ("MoleMaster just passed your score!")
  - Quiz countdown ("Quiz 3 in 2 days — you've covered 65%")
  - Achievement near-misses ("2 more correct answers for Molecule Master!")
  - Boss unlock ("You've mastered enough of Chapter 4 to challenge The Stereochemist!")
  - Session reminders (configurable daily reminder time)
- **Replicas**: 1-2
- **Rate limiting**: max 3 push notifications per day per user to avoid fatigue

---

## 5. Data Model

### 5.1 Postgres (Cloud SQL) — Source of Truth for PII + Relational Data

```sql
-- Core tables (simplified schema)

-- Users & Auth
users (id UUID PK, email, password_hash, display_name, avatar_emoji,
       account_type ENUM('standard','minor'), created_at, updated_at)
auth_tokens (id, user_id FK, refresh_token_hash, expires_at, device_info)

-- Subscriptions
subscriptions (id, user_id FK, stripe_customer_id, stripe_subscription_id,
               plan ENUM('monthly','quarterly','semesterly','trial'),
               status ENUM('active','past_due','cancelled','trial'),
               current_period_start, current_period_end, trial_end)

-- Learning Profiles (JSONB for flexibility)
learning_profiles (user_id FK PK, learning_style_preferences JSONB,
                   felder_silverman_dials JSONB, misconception_log JSONB,
                   explanation_preferences JSONB, updated_at)

-- Courses & Materials
courses (id, user_id FK, title, description, created_at)
materials (id, course_id FK, filename, gcs_path, file_type, processing_status,
           chunk_count, created_at)

-- Interaction History
interactions (id, user_id FK, course_id FK, session_id, role ENUM('student','tutor'),
              content TEXT, chunks_used JSONB, response_time_ms, created_at)

-- Assessments
quiz_results (id, user_id FK, course_id FK, topic, question TEXT,
              student_answer TEXT, correct_answer TEXT, is_correct BOOLEAN,
              time_taken_ms, created_at)

-- Gaming (persistent state)
gaming_profiles (user_id FK PK, level INT, xp BIGINT, total_questions INT,
                 correct_answers INT, current_streak INT, longest_streak INT,
                 bosses_defeated INT, gems INT, power_ups JSONB, updated_at)
achievements (id, user_id FK, achievement_type, earned_at)
boss_history (id, user_id FK, boss_name, topic, rounds, result ENUM('victory','defeat'),
              xp_earned, fought_at)

-- Chunk metadata
chunks (id UUID PK, material_id FK, course_id FK, content TEXT,
        chapter TEXT, section TEXT, page INT, content_type ENUM('text','table','equation','figure','quiz'),
        figure_gcs_path TEXT, metadata JSONB, created_at)

-- Concept knowledge graph (if using Postgres ltree instead of Neo4j)
concepts (id, course_id FK, name, description, path LTREE)
concept_prerequisites (concept_id FK, prerequisite_id FK, weight FLOAT)
student_concept_mastery (user_id FK, concept_id FK, mastery_score FLOAT,
                          last_reviewed_at, next_review_at, decay_rate FLOAT)

-- FERPA Audit Trail
audit_log (id, timestamp, accessor_id, student_id, action, data_accessed,
           purpose, ip_address)

-- Data export jobs
export_jobs (id, user_id FK, status ENUM('pending','processing','complete','failed'),
             gcs_path, created_at, completed_at)
```

**Row-Level Security**: Each student can only access their own data:
```sql
CREATE POLICY student_isolation ON interactions
    USING (user_id = current_setting('app.current_user_id')::uuid);
```

**pgvector columns**: Add to `interactions` and `learning_profiles` for semantic search over student's own history:
```sql
ALTER TABLE interactions ADD COLUMN embedding vector(1024);
```

### 5.2 Qdrant — Vector Search for Curriculum Content

**Collections**:

| Collection | Vectors | Payload Fields | Purpose |
|------------|---------|----------------|---------|
| `curriculum` | Dense (1024d, e5-large) + Sparse (BM25) | course_id, chapter, section, page, content_type, material_id | Main curriculum search |
| `concepts` | Dense (1024d) | concept_name, prerequisites, course_id, difficulty | Knowledge graph node embeddings |
| `insights` | Dense (1024d) | topic, insight_type, confidence, generated_at | Cross-student aggregate insights |
| `diagrams` (Phase 6) | Dense (768d, CLIP) | course_id, caption, page, figure_type | Multi-modal diagram search |

**Configuration**:
- 3 replicas for high availability
- HNSW index with scalar quantization (4x memory reduction)
- Snapshot schedule: nightly to GCS
- Payload indexing on `course_id` (most common filter)

### 5.3 Redis (Memorystore) — Real-Time Gaming State + Cache

| Key Pattern | Type | TTL | Purpose |
|-------------|------|-----|---------|
| `session:{userId}` | Hash | 24h | Active session context |
| `streak:{userId}` | String | — | Current streak count + last study date |
| `boss:{userId}:{bossId}` | Hash | 1h | Active boss battle state (HP, round, timer) |
| `leaderboard:global` | Sorted Set | — | Global XP leaderboard |
| `leaderboard:course:{courseId}` | Sorted Set | — | Per-course leaderboard |
| `quests:daily:{userId}` | Hash | 24h | Daily quest progress |
| `xp:today:{userId}` | String | 24h | Today's XP earned (for daily caps) |
| `cache:insight:{topic}` | String | 6h | Cached cross-student insight for topic |
| `ratelimit:{userId}:notif` | String | 24h | Notification rate limit counter |

### 5.4 Google Cloud Storage (GCS) — File Storage

| Bucket | Contents | Lifecycle |
|--------|----------|-----------|
| `tvtutor-raw-uploads` | Original uploaded files | 1 year, then Coldline |
| `tvtutor-processed` | Extracted figures, processed assets | Indefinite |
| `tvtutor-exports` | Student data export bundles | 30 days, then delete |
| `tvtutor-transcripts` | Session transcript archives | 1 year, then Coldline, 3 years delete |
| `tvtutor-backups` | Qdrant snapshots, Postgres WAL archives | 90 days |

### 5.5 BigQuery — Anonymized Analytics

| Table | Contents | Grain |
|-------|----------|-------|
| `topic_difficulty` | Error rates, common wrong answers per topic | Topic × day |
| `explanation_effectiveness` | Which explanation style led to correct follow-up | Style × topic |
| `learning_curves` | Mastery progression over time (anonymized cohorts) | Cohort × topic × week |
| `engagement_metrics` | Session length, questions per session, retention | Cohort × day |

---

## 6. RAG Pipeline Design

### 6.1 Agentic Retrieval Flow

The Tutoring Service uses a multi-step agentic RAG loop, not a single retrieve-then-generate step. The LLM acts as an agent that decides what to retrieve:

```
Student asks question
    │
    ▼
Step 1: Retrieve Student Knowledge Model (Postgres + pgvector)
    → Learning style preferences, current mastery state
    → Misconception log for this topic
    → Recent interaction context (last 5-10 exchanges)
    │
    ▼
Step 2: Query Concept Graph (Neo4j or Postgres ltree)
    → Does this question require prerequisite knowledge?
    → Has student mastered prerequisites? (check student_concept_mastery)
    → If gap detected → retrieve prerequisite content first
    │
    ▼
Step 3: Retrieve Curriculum Content (Qdrant hybrid search)
    → Dense vectors (semantic match to question)
    → Sparse vectors (keyword match for technical terms, formulas)
    → Filter by course_id, relevant chapters
    → Re-rank top results with cross-encoder
    → Return top 5-8 chunks with metadata
    │
    ▼
Step 4: Check Cross-Student Insights (cached from BigQuery)
    → "72% of students struggle with step 3 of this derivation"
    → "Visual explanation has 40% higher success rate for this topic"
    → "Common misconception: students confuse X with Y here"
    │
    ▼
Step 5: Generate Response via AI Gateway
    → System prompt includes: student's learning style dials,
      prerequisite status, misconception warnings, cross-student tips
    → Context includes: relevant curriculum chunks, student history
    → Adapt response format to learning style:
        Visual: lead with diagrams/models, color-coded elements
        Auditory: conversational tone, mnemonics, suggest TTS
        Kinesthetic: interactive challenges, "try building this"
        Read/Write: structured notes, definitions, flashcard prompts
    → Stream response token-by-token via SSE
    │
    ▼
Step 6: Post-Response Processing
    → Update SKM: mastery weights, interaction log, embedding
    → Check gaming triggers: XP award, quest progress, boss unlock
    → Schedule spaced repetition review for concepts covered
    → Log interaction for analytics pipeline
```

### 6.2 Retrieval Quality

The agentic approach enables:
- **Prerequisite-aware explanations**: if a student asks about derivatives but hasn't mastered limits, the tutor addresses the gap first
- **Misconception detection**: if the student's question implies a wrong mental model the tutor has seen before, it proactively corrects it
- **Multi-round retrieval**: the LLM may reformulate its search query after evaluating initial results ("the first results were about organic chemistry but the student is asking about biochemistry specifically")

### 6.3 RAG Evaluation

| Framework | Cadence | What It Measures |
|-----------|---------|-----------------|
| **RAGAS** (offline) | Weekly | Faithfulness (is answer grounded in context?), relevance (are retrieved docs relevant?), answer correctness |
| **Custom LLM judge** | Nightly | "Did this explanation address the student's actual question?" scored 1-5 |
| **Learning effectiveness** (north star) | Weekly | Correlation between tutoring interactions and subsequent quiz performance. "Did students who got this explanation do better on the quiz?" |
| **Production tracing** | Real-time | Latency per step, token usage, retrieval hit rate, via OpenTelemetry |

---

## 7. AI Gateway & Model Strategy

### 7.1 Self-Hosted AI Gateway

Deploy **LiteLLM** or **Portkey** (open-source core) on GKE as the AI gateway layer. All AI model calls route through this gateway.

**Why self-hosted**:
- Full K8s architecture — no external platform dependencies
- Cost tracking per student, per course, per interaction type
- Model failover (if Claude is down, route to GPT)
- Request/response logging for debugging and evaluation
- Rate limiting per user to control costs
- Guardrails integration (PII detection before sending to model providers)

### 7.2 AI Gateway Research Summary

| Gateway | OSS | Self-Hosted | Providers | Key Strength |
|---------|-----|-------------|-----------|-------------|
| **Vercel AI Gateway** | SDK only | No | 15+ | Zero-config on Vercel |
| **Portkey** | Core yes | Yes | 25+ | Guardrails (PII, content filtering) |
| **LiteLLM** | Yes | Yes | 100+ | OpenAI-compatible, broadest model support |
| **Helicone** | Yes | Yes | 15+ | Best observability/analytics |
| **OpenRouter** | No | No | 100+ | Simple multi-model access |
| **Cloudflare AI GW** | No | No | 10+ | Edge caching |
| **Martian** | No | No | 10-15 | Intelligent cost-aware routing |

**Recommendation**: Start with **LiteLLM** for broadest model compatibility and simplest self-hosting. Add **Portkey guardrails** when preparing for K-12 (content filtering, PII detection critical for minors).

### 7.3 Model Selection Strategy

| Use Case | Recommended Model | Why |
|----------|------------------|-----|
| Primary tutoring (explanations) | Claude Sonnet 4.6 or GPT-5.4 | Best at nuanced, patient explanations |
| Quiz question generation | Claude Haiku 4.5 or GPT-4.1-mini | Fast, cheap, structured output |
| Concept extraction (ingestion) | Claude Sonnet 4.6 | Best at understanding academic text structure |
| Embedding | e5-large-v2 (self-hosted) or text-embedding-3-large (OpenAI) | Self-hosted = no per-call cost at scale |
| Re-ranking | Cross-encoder (self-hosted) | Improves retrieval quality, runs on CPU |
| Boss taunts / sci-fi quotes | Claude Haiku 4.5 | Fun, creative, cheap |

**Provider-agnostic**: The AI Gateway abstracts all providers. Model strings like `anthropic/claude-sonnet-4.6` or `openai/gpt-5.4` can be swapped without code changes. A/B test models for tutoring quality.

---

## 8. Gaming System Design

### 8.1 Progression System

| Element | Mechanic | Purpose |
|---------|----------|---------|
| **XP** | Earned from every interaction: questions asked (+5), questions answered correctly (+15-50 based on difficulty), boss rounds (+50-100), daily quests (+25-75), streaks (+multiplier) | Core progression currency |
| **Levels** | XP thresholds: Novice (0) → Apprentice (500) → Scholar (2000) → Sage (5000) → Archmage (15000) → Legend (50000) | Visible rank, unlocks cosmetics |
| **Gems** | Premium currency from boss kills, streak milestones, achievements. Spent on power-ups, cosmetic character items | Engagement + scarcity mechanic |
| **Streaks** | Daily study streaks with escalating XP multiplier: 3-day = 1.5x, 7-day = 2x, 14-day = 2.5x, 30-day = 3x | Daily retention |
| **Daily Quests** | 3 quests per day: "Answer 5 questions", "Keep streak alive", "Master 1 new concept" | Session goals |
| **Achievements** | ~50 badges: "First Session", "7-Day Streak", "Quiz Master", "Boss Slayer", "Curious Cat (50 questions)", etc. | Collection / completionism |

### 8.2 Boss Battle System

Boss fights are the signature feature — animated, physics-driven encounters that make learning feel epic.

**Trigger**: Completing a mastery threshold (e.g., 60%+ on a chapter's concepts) unlocks the chapter boss.

**Mechanics**:
- **5-7 rounds** per boss fight, escalating difficulty
- **Timer**: 30-45 seconds per question (Time+ power-up adds 15 seconds)
- **HP system**: Student starts with 100 HP, boss HP scales with student's mastery level (adaptive difficulty)
- **Correct answer** = deal damage to boss (10-25 HP based on question difficulty)
- **Wrong answer** = boss deals damage to student (15-25 HP) + boss taunt
- **Combo streaks**: 3+ correct in a row = combo bonus (1.5x damage), 5+ = mega combo (2x)
- **Power-ups** (from gem shop or quest rewards):
  - 🧠 Hint: eliminates one wrong answer
  - 🛡️ Shield: blocks one wrong answer's damage
  - ⚡ 2x Damage: doubles XP for one correct answer
  - ⏰ Time+: adds 15 seconds to timer
- **Boss defeat** = XP reward (escalating per boss), loot drop (achievement badge, cosmetic item, gems), progression map advancement
- **Student defeat** (HP reaches 0) = no XP loss, can retry immediately, boss taunts with a comeback quote

**Visual design — Weird Science aesthetic**:
- Bosses are animated via WebGL/Three.js with real physics (Cannon.js or Rapier)
- Molecule bosses that morph and reconfigure when damaged
- Chemical reaction bosses with particle explosions, energy beams, glowing effects
- Screen shake on critical hits, particle bursts on correct answers
- Boss idle animations (pulsing, rotating, hovering)
- Death sequences with dramatic dissolution/explosion effects
- Background environments themed to the subject (periodic table arena, molecular lattice battlefield)

**Boss examples**:
| Boss | Subject | Visual | Signature Move |
|------|---------|--------|---------------|
| THE ATOM | Atomic structure | Orbiting electron shell entity | Fires electron beams |
| THE BONDER | Chemical bonding | Dual-headed molecule that splits/reforms | Creates false bonds to confuse |
| NAME LORD | Nomenclature | Shape-shifting compound that changes names | Rapid-fire naming rounds |
| THE STEREOCHEMIST | Stereochemistry | Mirror-image dragon that duplicates | Flips chirality mid-fight |
| THE REACTOR | Organic reactions | Chaotic reaction vessel | Cascading chain reaction attacks |
| ??? (Final Boss) | Course final | Revealed after all chapter bosses defeated | Multi-phase fight covering all topics |

### 8.3 Push Mechanics

| Trigger | Notification | Channel |
|---------|-------------|---------|
| Mastery below 50% on a topic | "Stereochem is at 42% — Prof. Nova has a 3x XP training program ready!" | Push + in-app |
| Simulated rival passes score | "MoleMaster just passed your score on Reactions! 😤" | In-app |
| Quiz approaching | "Quiz 3 in 2 days — you've covered 65% of the material" | Push + email |
| Streak at risk | "Study today to keep your 14-day streak! 🔥" | Push |
| Near achievement | "2 more correct answers for Molecule Master badge!" | In-app |
| Boss unlocked | "You've mastered enough of Chapter 4 to challenge The Stereochemist! ⚔️" | Push + in-app |
| Idle 3+ days | "Prof. Nova misses you! Your streak is safe for 24 more hours..." | Push + email |

**Rate limit**: max 3 push notifications per day.

### 8.4 Sci-Fi Quote System

Curated database of 200+ motivational quotes from science fiction, served contextually:

| Context | Example Quotes |
|---------|---------------|
| Session start | "The spice must flow." — Dune |
| Boss fight intro | "I aim to misbehave." — Firefly |
| Correct answer | "Punch it, Chewie!" — Star Wars |
| Wrong answer | "I've seen things you people wouldn't believe." — Blade Runner |
| Boss victory | "Do. Or do not. There is no try." — Yoda |
| Boss defeat | "Never tell me the odds." — Han Solo |
| Streak milestone | "The future is already here — it's just not evenly distributed." — William Gibson |
| Achievement | "Any sufficiently advanced technology is indistinguishable from magic." — Arthur C. Clarke |
| Comeback | "I am inevitable... and I am Iron Man." — Avengers |

Quotes rotate per session, never repeat in the same day. Stored in Postgres, cached in Redis.

### 8.5 Simulated Rivals

- AI generates 5-10 simulated competitor profiles with names, avatars, and progression curves
- Rivals advance at slightly slower/faster than the student to create dynamic competition
- Rival progress is algorithmic (based on student's pace ± variance), not a separate AI
- Designed to be replaced with real multiplayer leaderboards when user base is sufficient

---

## 9. Adaptive Learning System

### 9.1 Evidence-Based Core Strategies

These strategies are baked into the Tutoring Service's behavior, not optional:

| Strategy | Evidence Level | Implementation |
|----------|---------------|----------------|
| **Retrieval Practice** (testing effect) | Very strong | Tutor quizzes the student before re-explaining. "Before I explain, what do you remember about X?" |
| **Spaced Repetition** | Very strong | Scheduler tracks optimal review intervals per concept per student (SM-2 algorithm variant). Proactively surfaces review prompts. |
| **Interleaving** | Strong | Mix question types from different topics in quiz sessions and boss battles, rather than blocking by topic |
| **Elaborative Interrogation** | Strong | Tutor asks "Why does that make sense?" and "How does this connect to X?" after explanations |
| **Concrete Examples** | Strong | Always pair abstract concepts with specific, relatable examples |
| **Dual Coding** | Moderate-strong | Combine verbal explanation with visual representation (diagram + text) regardless of learning style |
| **Metacognitive Prompting** | Strong | "How confident are you?" before revealing answers. "What part is confusing?" to target gaps. |
| **Formative Feedback** | Very strong | Immediate, specific feedback on errors — not just "wrong" but WHY it's wrong |
| **Desirable Difficulties** | Strong | Questions slightly harder than current level. Scaffolding removed as mastery increases. |
| **Zone of Proximal Development** | Strong | Adaptive difficulty keeps challenges just beyond current ability with appropriate support |

### 9.2 Learning Style Adaptation

**Important research finding**: Learning style "matching" (only teaching in a student's preferred style) does NOT improve outcomes (Pashler et al., 2008). The evidence supports **multi-modal presentation with preference-informed emphasis**.

The tutor adapts along these dimensions (from Felder-Silverman + VARK):

| Dimension | Dial | How Tutor Adapts |
|-----------|------|-----------------|
| **Theory-first vs Example-first** | Sensing ↔ Intuitive | Start with "here's the principle" vs "here's a problem, let's figure out the principle" |
| **Big-picture vs Step-by-step** | Global ↔ Sequential | Overview first, then details vs ordered progression A→B→C |
| **Passive vs Active** | Reflective ↔ Active | "Consider this..." vs "Let's work a problem right now" |
| **Visual vs Verbal** | Visual ↔ Verbal | Lead with diagrams/models vs lead with text explanations |

**Detection**: The system observes interaction patterns, not a questionnaire:
- Does the student click diagrams or skip them? → visual preference signal
- Does the student ask "why" before "how"? → theory-first signal
- Does the student prefer practice problems or reading explanations? → active vs passive signal
- Response is always multi-modal — the preference determines EMPHASIS, not exclusion

### 9.3 Multi-Modal Response Formats

All four modes are always available. The tutor leads with the detected preference:

| Mode | What the Tutor Generates | UI Elements |
|------|-------------------------|-------------|
| **Visual** | 3D molecule models (WebGL), color-coded atom diagrams, side-by-side comparisons, concept maps, flowcharts | Interactive 3D viewer, rotate/zoom controls, animated transitions |
| **Auditory** | TTS audio of explanations, adjustable speed (0.5x-2x), key phrase replay, memory jingles/mnemonics, pronunciation guides | Audio player with waveform, speed controls, bookmarked phrases |
| **Kinesthetic** | Drag-and-drop molecule builders, step-by-step interactive labs, fill-in-the-blank derivations, "build before you learn" challenges | Interactive canvas, draggable elements, step tracker, reward on completion |
| **Read/Write** | Structured notes with definitions/rules/examples, auto-generated flashcards, Anki export, printable summaries, glossaries | Note panel, flashcard viewer, export buttons (PDF, Anki, Markdown) |

---

## 10. Student Knowledge Model (SKM)

The SKM is a persistent, evolving representation of each student's educational journey. It is the core competitive moat of TeachersLounge.

### 10.1 Components

| Component | Storage | Update Frequency | Purpose |
|-----------|---------|-----------------|---------|
| **Concept Mastery Graph** | Postgres (student_concept_mastery table) | Every interaction | Weighted graph of every topic studied. Mastery scores decay over time (spaced repetition curves). Edges show how the student connects concepts. |
| **Explanation Preference Model** | Postgres JSONB (learning_profiles) | Every 5-10 interactions | Which analogies, styles, and modalities worked for THIS student. E.g., "spatial analogies work for abstract math, step-by-step works for chemistry" |
| **Misconception Log** | Postgres JSONB (learning_profiles) | When detected | Persistent record of wrong mental models the student has exhibited. The tutor checks this before every response to avoid reinforcing errors. |
| **Interaction Embeddings** | Postgres pgvector (interactions table) | Every interaction | Vector representations of all Q&A sessions. Enables the tutor to reference past sessions: "Remember last semester when we covered X?" |
| **Notes & Highlights** | Postgres | Student-driven | Student-generated annotations linked to curriculum chunks. Searchable, exportable. |
| **Gaming History** | Postgres + Redis | Every gaming event | Full progression history — bosses fought, achievements earned, streak records. Part of the educational narrative. |

### 10.2 SKM Longevity & Monetization

The SKM is designed to persist for years. When a student studies organic chemistry in Year 1, then takes biochemistry in Year 3, the tutor remembers the foundations and builds on them.

**Open design decision — four options for SKM lifecycle**:

| Option | Description | Revenue Impact |
|--------|-------------|---------------|
| **A) Lives with subscription** | Active sub = full AI memory. Cancel = data archived for configured retention period, retrievable on re-subscription | Retention incentive — the longer you stay, the smarter your tutor gets. Strongest lock-in. |
| **B) SKM export as artifact** | Student can export their entire SKM as a portable file (JSON-LD + embeddings). Could theoretically import into another system. | One-time export fee, or included in premium tier. Supports data portability rights. |
| **C) Memory keeper tier** | After cancellation, student can access their SKM via read-only dashboard. Pay a lower "memory keeper" tier ($2-3/mo). | Ongoing revenue from churned users. Low cost to serve (no AI inference). |
| **D) Institutional licensing** | Universities/schools license aggregate anonymized SKMs to improve their curriculum design. | B2B revenue stream. Requires clear opt-in consent. |

These options are not mutually exclusive. Recommendation: implement A + B at launch, evaluate C + D based on churn patterns.

### 10.3 Data Export Format

Regardless of monetization decision, students can always export their personal data (FERPA/GDPR requirement):

```json
{
  "export_version": "1.0",
  "exported_at": "2026-03-29T12:00:00Z",
  "student": {
    "display_name": "ChemWizard",
    "account_created": "2025-09-01"
  },
  "courses": [
    {
      "title": "Organic Chemistry 101",
      "materials_uploaded": 12,
      "interactions": 847,
      "mastery": {
        "bonding": 0.92,
        "nomenclature": 0.71,
        "stereochemistry": 0.42
      }
    }
  ],
  "learning_profile": {
    "preferred_style": "visual-first",
    "felder_silverman": {
      "sensing_intuitive": 0.7,
      "visual_verbal": 0.8,
      "active_reflective": 0.4,
      "sequential_global": 0.6
    }
  },
  "interactions": [...],
  "quiz_results": [...],
  "notes": [...],
  "achievements": [...],
  "gaming_history": {
    "level": 12,
    "total_xp": 23400,
    "bosses_defeated": 12,
    "longest_streak": 14
  }
}
```

---

## 11. Subscription & Monetization

### 11.1 Plans

| Plan | Duration | Pricing (TBD) | Notes |
|------|----------|---------------|-------|
| Trial | 7-14 days | Free | Full access, converts to monthly on expiry |
| Monthly | 1 month | $XX/mo | Most flexible, highest per-month cost |
| Quarterly | 3 months | $XX/quarter | ~15% discount vs monthly |
| Semesterly | 6 months | $XX/semester | ~25% discount vs monthly, aligned with academic calendar |

### 11.2 Payment Processing

- **Stripe** for all payment processing
- Webhook-driven subscription lifecycle (create → renew → cancel → expire)
- Automatic dunning for failed payments (3 retry attempts over 7 days)
- Prorated upgrades/downgrades between plans
- Student discount verification (SheerID or similar) — future consideration

### 11.3 No Free Tier, No Ads

This is a deliberate design decision:
- No free tier eliminates freeloaders and ensures all users are invested
- No ads ensures a distraction-free learning environment (critical for users who are distractable, per the product requirements)
- The trial period provides the "try before you buy" experience
- The gamification (streaks, XP, bosses) creates organic retention without ads

---

## 12. Data Privacy & Compliance

### 12.1 FERPA Compliance

| Requirement | Implementation |
|-------------|---------------|
| Written consent before disclosing PII | Consent flow in onboarding. Signed consent records stored in Postgres. |
| Legitimate educational interest access | Role-based access control (RBAC). All access logged in audit_log table. |
| Right to inspect and amend records | Student data dashboard (read + edit). Accessible via profile settings. |
| Annual notification of rights | Terms of service + annual email notification. |
| Audit trail of disclosures | Postgres audit_log table: timestamp, accessor, student_id, action, data_accessed, purpose, IP. |

### 12.2 GDPR Readiness

| Requirement | Implementation |
|-------------|---------------|
| Lawful basis | Consent (for tutoring services) and legitimate interest (for anonymized analytics) |
| Right to erasure | Cascading delete across all services: Postgres, Qdrant (delete by user_id payload filter), GCS, Redis. Triggered via `DELETE /users/{id}`. |
| Right to portability | JSON export endpoint (Section 10.3). Automated background job → GCS → download link. |
| Data minimization | Only store what's needed. Purge session transcripts after retention period. |
| DPA with processors | Data Processing Agreements with Google Cloud, AI model providers (OpenAI, Anthropic). |
| Data residency | `europe-west` GKE/Cloud SQL regions for EU students. Region selection at registration. |

### 12.3 Data Retention Policy

| Data Type | Active Retention | After Account Closure | Final Disposition |
|-----------|-----------------|----------------------|------------------|
| Student profile | Active enrollment | 3 years | Delete or anonymize |
| Interaction history | Active enrollment | 2 years | Anonymize and aggregate to BigQuery |
| Session transcripts | 1 year | Delete | — |
| Assessment/quiz results | Active enrollment | 5 years (accreditation) | Archive to Coldline, then delete |
| Uploaded materials | Active enrollment | 90 days after closure | Delete from GCS |
| Gaming history | Active enrollment | 3 years | Anonymize |
| Aggregated analytics | Indefinite (anonymized) | N/A | Indefinite |
| Audit logs | 7 years (compliance) | 7 years | Archive to Coldline |

### 12.4 Encryption

| Layer | Mechanism |
|-------|-----------|
| At rest — GKE disks | AES-256, Google-managed keys. Upgrade to CMEK (Customer-Managed Encryption Keys) via Cloud KMS. |
| At rest — Cloud SQL | Encrypted by default. CMEK available. |
| At rest — GCS | Encrypted by default. CMEK available. |
| In transit — internal | Istio mTLS or GKE Dataplane V2 (WireGuard). Cloud SQL SSL enforced. |
| In transit — external | TLS 1.3, HSTS headers. |
| Application-level PII | Envelope encryption (Cloud KMS wraps DEK). Protects against database dump exposure. |

### 12.5 Student Departure Workflow

1. Student requests account deletion (or subscription expires past retention window)
2. Generate full data export (Section 10.3) and email download link
3. Anonymize: replace PII with hashed identifiers in analytics tables
4. Delete: purge from Postgres (cascading), Qdrant (delete by user_id filter), GCS (raw uploads, transcripts), Redis (session data)
5. Retain: audit logs (with redacted PII) for compliance period
6. Verify: sweep job confirms no orphaned data across all stores

---

## 13. UI/UX Design — Neon Arcade

### 13.1 Visual Identity

- **Palette**: Neon blue (#00aaff), neon green (#00ff88), neon pink (#ff00aa), gold (#ffdc00) on deep dark (#0a0a1a)
- **Aesthetic**: Rave / arcade / cyberpunk with Weird Science overtones
- **Glow effects**: Radial gradients, box shadows with neon colors, text shadows on key elements
- **Typography**: Geist Sans for UI, Geist Mono for code/metrics/XP counts
- **Motion**: Subtle pulse animations on active elements, smooth transitions, particle effects in boss battles

### 13.2 Layout — Neon Arcade (Three-Panel Immersive)

```
┌─────────────────────────────────────────────────────────────┐
│ [TV Logo]  [🤖 Prof Nova ●]     [🔥 7] [⚡ 2.3k] [💎 450] [🧙 Avatar] │
├──────────────┬──────────────────────────┬───────────────────┤
│              │                          │                   │
│  CHARACTER   │       CHAT PANEL         │   MATERIALS /     │
│  & QUESTS    │                          │   MASTERY /       │
│              │  [Tutor messages]        │   LEADERBOARD     │
│  Avatar      │  [Student messages]      │                   │
│  Level/XP    │  [Inline quizzes]        │  [Doc viewer]     │
│  Streak      │  [Boss battle UI]        │  [Mastery bars]   │
│  Daily Quests│  [Achievement toasts]    │  [Power-ups]      │
│  Achievements│                          │  [Rankings]       │
│  Materials   │  [Input: "Ask Prof       │                   │
│  List        │   Nova anything..." ⚡]  │                   │
│              │                          │                   │
├──────────────┴──────────────────────────┴───────────────────┤
│ [XP Progress Bar ████████████░░░░░░░░░░░ 65% to Level 13]  │
└─────────────────────────────────────────────────────────────┘
```

### 13.3 Key UI Components

| Component | Description |
|-----------|-------------|
| **Character Card** | Student avatar (emoji-based initially, custom art later), class name, rank title, XP bar, achievement badges |
| **Professor Nova** | Tutor avatar in chat bubbles, contextual mood (encouraging, challenging, celebrating), animated in boss fights |
| **Chat Panel** | Streaming AI responses with rich formatting (KaTeX for math, syntax highlighting for code, inline diagrams). Learning mode badges per message (👁️ Visual, 🎧 Audio, 🖐️ Hands-On, 📝 Study) |
| **Boss Battle Overlay** | Full-screen takeover with WebGL canvas: boss character, HP bars, timer, power-up tray, battle log, sci-fi quote |
| **Mastery Map** | Progress bars per topic with color coding: red (<40%), yellow (40-70%), green (>70%), gold (>90% mastered) |
| **Quest Board** | Daily quests with progress indicators, reward previews |
| **Leaderboard** | Top 10 + student's rank, simulated rival highlights |
| **Achievement Gallery** | Grid of earned/locked badges with glow effect on new unlocks |
| **Document Viewer** | Split-pane view of uploaded material with highlights linked to chat context |
| **Audio Player** | TTS playback with waveform, speed control, key phrase bookmarks |
| **Molecule Builder** | Drag-and-drop interactive canvas for kinesthetic learning (Three.js) |

### 13.4 Responsive Design

- **Desktop (>1200px)**: Full three-panel layout
- **Tablet (768-1200px)**: Chat + one collapsible side panel
- **Mobile (<768px)**: Tab-based navigation (Chat | Materials | Progress | Profile). Boss battles adapt to portrait mode.

---

## 14. Phased Build Plan

### Phase 1: Foundation (Weeks 1-6)

**Goal**: Student can sign up, subscribe, and chat with an AI tutor.

| Deliverable | Details |
|-------------|---------|
| GKE cluster setup | Autopilot or Standard, VPC, Cloud NAT, Artifact Registry, CI/CD pipeline (GitHub Actions → ArgoCD) |
| Frontend Service | Next.js in Docker, Neon Arcade shell (layout, nav, theming), basic chat UI |
| User Service | Registration, JWT auth, profile CRUD |
| Tutoring Service | Basic chat with AI (no RAG yet), streaming SSE, conversation history |
| AI Gateway | LiteLLM deployed on GKE, single provider configured |
| Postgres (Cloud SQL) | Users, auth, profiles, interactions, subscriptions tables |
| Stripe integration | Monthly/quarterly/semesterly plans, trial period, webhook handler |
| Redis (Memorystore) | Session cache |

**Exit criteria**: A student can register, start a trial, chat with Prof. Nova, and subscribe.

### Phase 2: RAG Core (Weeks 5-10)

**Goal**: Student uploads a textbook PDF, tutor answers questions grounded in that material.

| Deliverable | Details |
|-------------|---------|
| Ingestion Service | PDF processing (unstructured.io), hierarchical chunking, embedding generation |
| Qdrant deployment | 3-replica Helm chart, curriculum collection |
| Search Service | Hybrid search (dense + sparse), payload filtering |
| Agentic RAG flow | Multi-step retrieval in Tutoring Service |
| Material upload UI | File upload, processing status, material library |
| Chunk metadata tables | Postgres schema for chunk registry |

**Exit criteria**: Upload a chemistry textbook PDF, ask "what is a chiral center?", get an answer grounded in the textbook with page references.

### Phase 3: Gaming Layer (Weeks 9-14)

**Goal**: Full gamification — XP, streaks, quests, leaderboard, achievements.

| Deliverable | Details |
|-------------|---------|
| Gaming Service | XP engine, level progression, streak tracker, daily quest generator |
| Redis gaming state | Leaderboards (sorted sets), boss state, streak counters |
| Character system | Avatar selection, class/rank display, achievement gallery |
| Quiz system | AI-generated quizzes from curriculum, scoring, XP awards |
| Simulated rivals | Algorithmic competitor profiles |
| Sci-fi quote database | 200+ curated quotes, context-aware serving |
| Push mechanics | Weak spot alerts, rival notifications, streak warnings (in-app) |
| Neon Arcade UI polish | Full three-panel layout, gamification widgets, animations |

**Exit criteria**: Student earns XP from studying, levels up, has daily quests, sees leaderboard with simulated rivals.

### Phase 4: Boss Battles (Weeks 13-18)

**Goal**: Animated boss fights at chapter completion with physics and Weird Science aesthetic.

| Deliverable | Details |
|-------------|---------|
| WebGL boss engine | Three.js rendering, Cannon.js/Rapier physics |
| Boss character library | 5-6 animated bosses with idle/attack/damage/death animations |
| Battle state machine | Round management, HP tracking, power-up consumption, combo system |
| Boss progression map | Visual trail of defeated/current/locked bosses |
| Boss taunts | AI-generated contextual taunts |
| Power-up shop | Gem-based purchasing of hints, shields, 2x damage, time+ |
| Weird Science effects | Particle systems, screen shake, glow effects, morphing molecules |
| Loot drops | Rewards on boss defeat (badges, gems, cosmetics) |

**Exit criteria**: Complete a chapter, fight an animated boss with molecule physics, earn loot on victory.

### Phase 5: Adaptive Learning (Weeks 17-22)

**Goal**: Tutor adapts to student's learning style, remembers past interactions, schedules reviews.

| Deliverable | Details |
|-------------|---------|
| Student Knowledge Model | Concept mastery graph, explanation preferences, misconception log |
| Learning style detection | Behavioral analysis of interaction patterns → Felder-Silverman dials |
| Multi-modal responses | Visual (diagrams), auditory (TTS), kinesthetic (builders), read/write (notes) |
| Spaced repetition scheduler | SM-2 variant, proactive review prompts |
| Concept knowledge graph | Neo4j or Postgres ltree, prerequisite relationships |
| Prerequisite-aware tutoring | Detect gaps, address foundations before advancing |
| Flashcard system | Auto-generated flashcards, Anki export |

**Exit criteria**: Tutor detects visual learning preference, leads with diagrams, schedules spaced reviews, references past sessions.

### Phase 6: Full Ingestion (Weeks 21-26)

**Goal**: Students can upload any file type — docs, slides, video lectures, audio recordings.

| Deliverable | Details |
|-------------|---------|
| Office doc processing | python-docx, python-pptx, openpyxl → HTML → chunking |
| Video/audio transcription | Whisper (self-hosted on GPU node) or Google Speech-to-Text |
| Image OCR | Google Document AI for scanned PDFs, handwritten notes |
| Multi-modal RAG | CLIP embeddings for diagrams, image-based retrieval |
| LaTeX support | Equation parsing, KaTeX rendering |
| Molecule builder | Interactive Three.js canvas for kinesthetic chemistry learning |

**Exit criteria**: Upload a video lecture, PowerPoint, and handwritten notes — tutor can answer questions from all three.

### Phase 7: Analytics + Privacy (Weeks 25-30)

**Goal**: Cross-student analytics improving tutor quality, full compliance posture.

| Deliverable | Details |
|-------------|---------|
| Analytics Service | Nightly CronJob: anonymize → aggregate → BigQuery |
| Cross-student insights | Feed aggregate patterns back into tutoring prompts |
| FERPA audit trail | Complete audit_log implementation, access reporting |
| GDPR compliance | Right to erasure workflow, data export endpoint, consent management |
| Encryption hardening | CMEK for Cloud SQL/GCS, application-level PII encryption |
| Data retention automation | Lifecycle policies, archival jobs, deletion verification sweeps |
| RAGAS evaluation suite | Weekly RAG quality metrics, learning effectiveness tracking |
| Data export | Full student data export (JSON), automated background job |

**Exit criteria**: Analytics dashboard shows cross-student insights, complete FERPA audit, successful data export/deletion test.

### Phase 8: Polish + Scale (Weeks 29-34)

**Goal**: Production-ready platform with notification-driven engagement and monitoring.

| Deliverable | Details |
|-------------|---------|
| Notification Service | Push (FCM), email (SendGrid/Resend), streak reminders, rival alerts |
| K-12 architecture hooks | Account type field, parental consent flow skeleton, age gate (not fully implemented) |
| Performance optimization | Load testing (k6), query optimization, caching layer tuning |
| Monitoring + alerting | Prometheus/Grafana dashboards, PagerDuty integration, SLOs |
| Security hardening | Penetration testing, dependency scanning, CSP headers |
| Mobile responsiveness | Tab-based mobile layout, touch-optimized boss battles |
| Onboarding flow | First-run tutorial, character creation wizard, first material upload guide |

**Exit criteria**: Production deployment with monitoring, <2s TTFB, notification-driven re-engagement, onboarding flow complete.

**Total estimated scope: ~28-34 weeks** (phases overlap by 2-4 weeks with parallel workstreams).

---

## 15. Open Design Decisions

These decisions are deferred to implementation time. Each requires further discussion or data:

| # | Decision | Options | Notes |
|---|----------|---------|-------|
| 1 | SKM monetization model | A) Lives with sub, B) Exportable artifact, C) Memory keeper tier ($2-3/mo), D) Institutional licensing | Not mutually exclusive. Recommend A+B at launch. |
| 2 | Data export pricing | Free with sub, one-time fee, premium tier only | FERPA/GDPR require free access to personal data. Monetization is for the SKM artifact (trained model), not raw data. |
| 3 | K-12 integration timeline | Phase 8 skeleton hooks, full implementation as Phase 9+ | COPPA adds significant complexity. Defer until post-launch. |
| 4 | Default AI provider for tutoring | Claude Sonnet 4.6, GPT-5.4, or Gemini | A/B test via AI Gateway. Let data decide. |
| 5 | Neo4j vs Postgres ltree for concept graph | Neo4j (dedicated graph DB) vs Postgres ltree (keep it in Postgres) | Start with Postgres ltree. Migrate to Neo4j if graph queries become complex (>3 hops). |
| 6 | Self-hosted embedding model vs API | e5-large-v2 on GPU node ($200-400/mo) vs OpenAI API ($0.13/M tokens) | API for Phase 1-2 (faster iteration). Self-host at scale (Phase 6+). |
| 7 | Boss battle art style | Commissioned character art vs procedurally generated | Commissioned is higher quality, procedural is more scalable. Hybrid: commission key bosses, procedural for variants. |
| 8 | Real multiplayer timeline | Post-launch when user base supports it | Simulated rivals cover the gap. Real multiplayer needs matchmaking, anti-cheat, moderation. |
? 9 | Subscription pricing | Market research needed | Competitors: Chegg ($15/mo), Course Hero ($10/mo), Khan Academy (free). TeachersLounge's gaming + personalization justifies premium. |
| 10 | Audio mode TTS provider | Google Cloud TTS, ElevenLabs, OpenAI TTS | ElevenLabs highest quality but most expensive. Google TTS good enough for launch. |

---

## 16. Research Appendices

### Appendix A: AI Gateway Comparison

| Gateway | OSS | Self-Hosted | Providers | Routing/Failover | Observability | Caching | Cost Tracking | Guardrails |
|---------|-----|-------------|-----------|------------------|---------------|---------|---------------|------------|
| Vercel AI Gateway | SDK only | No | 15+ | Yes | Yes | Yes | Yes | No |
| Portkey | Core yes | Yes | 25+ | Advanced | Yes | Yes | Yes | Yes (PII, content) |
| LiteLLM | Yes | Yes | 100+ | Yes | Basic | Yes | Yes | No |
| Helicone | Yes | Yes | 15+ | Limited | Best-in-class | Yes | Yes | No |
| Martian | No | No | 10-15 | Intelligent | Limited | No | Yes | No |
| OpenRouter | No | No | 100+ | Basic | Basic | No | Yes | No |
| Cloudflare AI GW | No | No | 10+ | Limited | Yes | Yes | Yes | No |

**Selected**: LiteLLM for launch (broadest model support, simplest self-hosting, OpenAI-compatible API). Add Portkey guardrails layer when preparing for K-12.

### Appendix B: Learning Styles Research

#### Models Evaluated

**VARK (Fleming & Mills, 1992)**: Visual, Auditory, Read/Write, Kinesthetic. Most widely known. Easy to adapt output format. Evidence for matching hypothesis: **weak** (Pashler et al., 2008; Rogowsky et al., 2015).

**Kolb's Experiential Learning (1984)**: Diverging, Assimilating, Converging, Accommodating. Based on 2 axes: Concrete/Abstract × Active/Reflective. Key lever for AI: **sequencing** (theory-first vs. practice-first). Evidence: **mixed**.

**Gardner's Multiple Intelligences (1983)**: 8-9 types (Linguistic, Logical-Mathematical, Spatial, Musical, Bodily-Kinesthetic, Interpersonal, Intrapersonal, Naturalistic, Existential). Several are hard to leverage in a chatbot. Linguistic, Logical-Mathematical, Interpersonal, and Intrapersonal are actionable. Evidence as instructional differentiation tool: **weak**.

**Honey & Mumford (1982)**: Activist, Reflector, Theorist, Pragmatist. Maps to conversation strategies. Evidence: **limited peer review**.

**Felder-Silverman (1988)**: 4 bipolar dimensions — Sensing/Intuitive, Visual/Verbal, Active/Reflective, Sequential/Global. **Most actionable for AI tutor** because each dimension maps to a concrete instructional choice. Evidence: **moderate** (reasonable psychometric properties).

**Dunn & Dunn (1978)**: 5 categories (Environmental, Emotional, Sociological, Physiological, Psychological). Most variables outside AI's control. Evidence: **methodological concerns**.

**Cognitive Style Dimensions**: Field Dependent/Independent (Witkin), Holist/Serialist (Pask), Deep/Surface Processing (Marton & Säljö). Deep vs. Surface processing is highly actionable — deep processing is reliably superior.

#### Evidence-Based Strategies (What Actually Works)

| Strategy | Evidence Level | AI Tutor Application |
|----------|---------------|---------------------|
| Retrieval Practice (testing effect) | Very Strong | Quiz before re-explaining |
| Spaced Repetition | Very Strong | Schedule reviews at increasing intervals |
| Interleaving | Strong | Mix problem types |
| Elaborative Interrogation | Strong | "Why does that make sense?" |
| Concrete Examples | Strong | Always pair abstractions with specifics |
| Dual Coding | Moderate-Strong | Combine verbal + visual |
| Scaffolding / ZPD | Strong | Adaptive difficulty |
| Metacognitive Prompting | Strong | "How confident are you?" |
| Formative Feedback | Very Strong | Immediate, specific error feedback |
| Desirable Difficulties (Bjork) | Strong | Slightly harder than current level |

#### Key Insight

Learning style *preferences* are real — people do prefer certain formats. Learning style *matching* as a way to improve outcomes is **not supported by evidence**. The correct approach is Universal Design for Learning (UDL): present in multiple modalities, offer choice, use evidence-based strategies as the core pedagogy, and treat preference as one input among many.

### Appendix C: RAG Architecture Research

#### Three Pipelines

| Pipeline | What It Indexes | Query Pattern | Storage | Update Frequency | Data Sensitivity |
|----------|----------------|--------------|---------|-----------------|-----------------|
| Curriculum Content RAG | Textbooks, PDFs, syllabi, quizzes, homework | Student question → retrieve relevant passages | Qdrant (dense + sparse vectors) | Batch on upload | Medium (copyrighted material) |
| Student Profile RAG | Per-student interaction history, learning preferences | Before response: retrieve student context | Postgres + pgvector | Every interaction | High (PII, FERPA) |
| Cross-Student Analytics | Anonymized aggregate patterns | Batch analytics queries | BigQuery + cached in Qdrant/Postgres | Nightly batch | Low (anonymized) |

#### Vector Database Selection

**Qdrant** selected for curriculum content:
- Single Rust binary, ~200MB RAM for 1M vectors
- Official Helm chart for GKE (production-ready)
- Native hybrid search (dense + sparse vectors)
- Payload filtering (chapter, course, content type)
- Snapshots to GCS for backups
- Cost: ~$50-100/mo self-hosted on GKE (3 replicas)

**pgvector** selected for student profiles:
- Keeps PII in Postgres with row-level security
- ACID transactions (update vector + profile atomically)
- AlloyDB on GCP accelerates pgvector 10x
- Fine for <5M vectors (student-scale, not curriculum-scale)

#### Document Processing

| File Type | Tool | Notes |
|-----------|------|-------|
| PDF (digital) | unstructured.io | Layout-aware parsing |
| PDF (scanned) | Google Document AI | Best OCR for complex layouts |
| Handwriting | Document AI (handwriting model) | Best-in-class |
| Office docs | python-docx/pptx/openpyxl | Convert to HTML first |
| Video | ffmpeg → Whisper/Speech-to-Text | Extract audio, then transcribe |
| Audio | Whisper/Speech-to-Text | Whisper large-v3 comparable quality |
| LaTeX | Pandoc/custom parser | Preserve equation structure |

#### Emerging Techniques

- **Agentic RAG**: Multi-step retrieval with LLM deciding what to search. Recommended from day one for tutoring quality.
- **GraphRAG**: Knowledge graphs for prerequisite-aware explanations. Build lightweight concept graph early.
- **Hybrid Search**: Dense + sparse vectors in Qdrant. Captures both meaning and exact terms.
- **Multi-modal RAG**: CLIP embeddings for diagram retrieval. Phase 6.
- **RAG Evaluation**: RAGAS (weekly), custom learning effectiveness metric (north star).

### Appendix D: Data Privacy Requirements

See Section 12 for full implementation details. Key frameworks:
- **FERPA**: US education data. Consent, access control, audit trail, right to inspect/amend.
- **GDPR**: EU data. Right to erasure, portability, data minimization, DPAs, residency.
- **COPPA**: Under-13. Deferred to K-12 phase. Architecture designed to support parental consent flows.

---

*End of specification. This document should be reviewed and approved before proceeding to implementation planning.*
