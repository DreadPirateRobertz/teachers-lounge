# Embedding Model Decision: OpenAI text-embedding-3-large vs e5-large-v2 Self-Hosted

**Date**: 2026-03-29
**Status**: Decision — Confirmed with modifications
**Author**: dink (crew, teachers_lounge)
**Gates**: tl-sui (Ingestion Service implementation)

---

## TL;DR

**Use OpenAI text-embedding-3-large for Phase 2.** The spec's API-first recommendation holds, but the break-even point is Phase 4–5 (≈30–50K users), not Phase 6. The latency profile also warrants early investment in self-hosted infrastructure during Phase 3 to avoid the query latency ceiling.

---

## Options Evaluated

| | **Option A: OpenAI API** | **Option B: Self-Hosted e5-large-v2** |
|---|---|---|
| Model | `text-embedding-3-large` (1024d output) | `e5-large-v2` (1024d native) |
| Hosting | OpenAI API | GKE `gpu-pool`: g2-standard-4 + NVIDIA L4 |
| Ops burden | Zero | Medium (0.5–1 week setup + ongoing) |
| Cost model | Per-token variable | Fixed ~$408/mo (1yr committed) |
| Phase 2 monthly cost | ~$10–15 | $408 |

---

## 1. Quality: MTEB Benchmarks

| Model | MTEB Avg (56 tasks) | Retrieval Avg | Notes |
|-------|--------------------|--------------------|-------|
| text-embedding-3-large (3072d) | 64.6 | 55.4 | Full dimensions |
| text-embedding-3-large (1024d) | 64.1 | 54.9 | Truncated — minimal quality loss |
| e5-large-v2 (1024d) | 62.2 | 50.6 | Older architecture |
| e5-mistral-7b-instruct (4096d) | 66.6 | 56.9 | Better but 3B params, heavy ops |

**Verdict**: OpenAI has a meaningful ~2.4pt MTEB advantage and a notable ~4pt retrieval advantage over e5-large-v2. For academic text (our domain), retrieval quality matters. The 1024d truncated output retains nearly full quality.

If self-hosting is chosen at scale, prefer a more modern model (e.g., `bge-large-en-v1.5` or `gte-large`) over e5-large-v2 — the quality gap with OpenAI narrows considerably with newer models.

---

## 2. Cost Analysis

### Assumptions

| Parameter | Value | Basis |
|-----------|-------|-------|
| Avg chunk size | 768 tokens | 512–1024 range, hierarchical chunks |
| Avg query size | 50 tokens | Typical student question |
| Ingestion rate | 10% of users upload new material/month | Conservative estimate |
| Chunks per upload | 500 chunks | ~500-page course pack per semester |
| DAU rate | 50% of registered users | Engaged platform assumption |
| Queries per DAU | 20/day | Active tutoring session |
| OpenAI price | $0.13/1M tokens | `text-embedding-3-large` (current) |
| GPU node (committed) | $408/mo | g2-standard-4, 1yr committed, us-central1 |

### Monthly Cost by Scale

| Scale | Registered Users | OpenAI Ingestion | OpenAI Queries | **OpenAI Total** | **GPU Fixed** |
|-------|-----------------|-----------------|----------------|-----------------|---------------|
| Phase 2 | 500 | $4 | $5 | **~$9** | **$408** |
| 5K users | 5,000 | $25 | $49 | **~$74** | **$408** |
| 10K users | 10,000 | $50 | $98 | **~$148** | **$408** |
| 30K users | 30,000 | $150 | $293 | **~$443** | **$408–816** |
| 50K users | 50,000 | $250 | $488 | **~$738** | **$816** |
| 100K users | 100,000 | $500 | $975 | **~$1,475** | **$1,224** |

**Break-even: approximately 30–35K registered users** (where OpenAI ≈ 1× GPU node).

At 50K+ users, self-hosting is clearly cheaper. Note: one L4 GPU handles ~2,000–4,000 embeddings/sec — sufficient for up to ~100K users with single-replica deployment.

### One-Time Ingestion Cost (historical data)

At migration/transition point (full corpus re-embedding if switching models):
- 1M chunks × 768 tokens = 768M tokens = **$100** via OpenAI

Re-embedding at transition is negligible in cost.

---

## 3. Latency Profile

This is the most important non-cost consideration.

| | OpenAI API | e5-large-v2 Self-Hosted |
|--|------------|------------------------|
| Embedding latency | 50–150ms (network round-trip) | 5–20ms (in-cluster) |
| Cold start | N/A | 2–3 min (pod scheduling + model load) |
| Batch throughput | ~300 RPS (rate limited) | ~2,000–4,000/sec (L4) |

### Query Latency Budget (Search Service target: <200ms)

| Step | OpenAI | Self-Hosted |
|------|--------|-------------|
| Query embedding | 50–100ms | 10–20ms |
| Qdrant hybrid search | 20–50ms | 20–50ms |
| Cross-encoder re-ranking (top 10) | 50–100ms | 50–100ms |
| **Total estimate** | **120–250ms** | **80–170ms** |

**OpenAI puts the 200ms target at risk.** Under typical P95 conditions, embedding round-trips can reach 150ms+, pushing total search latency above the target. Self-hosted is comfortably within budget.

**Mitigation for Phase 2 (OpenAI)**:
- Cache embeddings of frequently repeated queries (Redis, 6h TTL) — cuts hot-path API calls by ~30–50%
- Pre-embed all stored chunk content at ingestion time (no re-embedding at query time — only the query itself goes to API)
- Monitor P95 embedding latency in production; if >100ms consistently, trigger early self-hosted migration

---

## 4. Operational Burden

### OpenAI (Option A)

| Task | Effort |
|------|--------|
| Setup | Add API key to env; install `openai` SDK | Zero |
| Monitoring | Track latency + cost via API dashboard | Minimal |
| Updates | Nothing to update | Zero |
| Failure mode | API down → ingestion paused, queries fail | Mitigated via retry logic |

**Total ops cost: ~0 engineering hours/week.**

### Self-Hosted e5-large-v2 (Option B)

| Task | Effort |
|------|--------|
| Initial setup | Triton or Hugging Face `text-embeddings-inference` container, GKE Deployment, GPU node pool | 3–5 days |
| Helm chart | Write/maintain chart, resource tuning | 1 day |
| Monitoring | GPU utilization, model latency, OOM alerts | 0.5 days setup + ongoing |
| Model updates | Evaluate new models, re-embed corpus | 1–2 days/year |
| Incident response | GPU node issues, OOM kills, model serving errors | Unpredictable |

**Total initial investment: ~1 week. Ongoing: ~2–4 hrs/week.** Significant cost for a 2–5 person team.

---

## 5. FERPA & Compliance

| Concern | OpenAI | Self-Hosted |
|---------|--------|-------------|
| Student query content leaves infrastructure | Yes (query text sent to OpenAI) | No |
| FERPA "school official" coverage | OpenAI DPA available; covers API use | Not applicable |
| GDPR data residency | OpenAI US/EU processing regions | GKE region of your choice |
| Data retention | OpenAI: 0 days for API inputs (API TOS) | In your control |

**Assessment**: OpenAI's risk level for Phase 2 is **low but non-zero**. Student query text (which may contain course-specific information but not direct PII) is sent to OpenAI. OpenAI's API terms provide zero-day retention and prohibit training on API inputs, which satisfies the core FERPA concern.

**Action item**: Obtain legal sign-off on OpenAI DPA for FERPA compliance before Phase 2 launch. This is a 1-2 day task, not a blocker.

Self-hosted eliminates this concern entirely, which is the cleaner compliance posture.

---

## 6. Phase 2 Rollout Speed

| | OpenAI | Self-Hosted |
|--|--------|-------------|
| Time to first embedding in production | 1 hour (API key + SDK call) | 1–2 weeks |
| Time to stable production | 1 day | 2–3 weeks |

Phase 2 can ship 2–3 weeks faster with OpenAI. For a small team in early build, this is significant.

---

## 7. Decision Matrix

| Factor | Weight | OpenAI Score | Self-Hosted Score |
|--------|--------|-------------|------------------|
| Phase 2 cost | High | 5/5 (10× cheaper) | 1/5 (overkill fixed cost) |
| Retrieval quality | High | 5/5 | 3/5 |
| Query latency | High | 3/5 (tight on budget) | 5/5 |
| Ops burden | High | 5/5 | 2/5 |
| Rollout speed | High | 5/5 | 2/5 |
| FERPA compliance | Medium | 3/5 (needs DPA) | 5/5 |
| Scale economics | Low (Phase 2) | 3/5 (breakeven at 30K) | 4/5 |
| **Weighted score** | | **4.1/5** | **3.0/5** |

---

## 8. Recommendation

### Phase 2 (current): Use OpenAI text-embedding-3-large

**Rationale**: 10× cost advantage, zero ops overhead, faster rollout, better quality. The spec's recommendation is correct.

**Non-negotiables with OpenAI**:
1. **Cache query embeddings in Redis** (6h TTL, hash key on query text + course_id). Reduces API latency on repeated queries and cuts cost.
2. **Obtain OpenAI DPA** and confirm FERPA coverage before onboarding paying students.
3. **Instrument embedding latency** from day one. Alert if P95 > 100ms.
4. **Do not use 3072d** — request 1024d output via `dimensions` parameter. Memory savings in Qdrant, negligible quality loss.

### Phase 3–4 (est. 10K–30K users): Prepare self-hosted infrastructure

The gpu-pool node type (`g2-standard-4 + NVIDIA L4`) is already in the spec. During Phase 3:
1. Deploy `text-embeddings-inference` (Hugging Face) with a modern model (evaluate `bge-large-en-v1.5` or equivalent MTEB leader at the time)
2. Run in shadow mode alongside OpenAI — compare quality and latency
3. Build migration path: Qdrant re-indexing job, model version pinning

**Migration trigger** (whichever comes first):
- Registered users exceed 30K
- P95 embedding latency > 100ms consistently (latency trigger)
- Monthly OpenAI embedding cost exceeds $300

### Phase 4–5 (est. 30K–50K users): Cut over to self-hosted

Full cutover to in-cluster embedding. Re-embed full corpus (cost: ~$100–200 one-time via OpenAI if using it for the final batch, otherwise just local CPU batch job). Keep OpenAI as fallback for GPU outage scenarios.

### Phase 6+ (original spec): Keep self-hosted, evaluate newer models

At scale, also evaluate whether `e5-large-v2` is still the right self-hosted choice. By Phase 6, there will be newer, more efficient models. The Qdrant collection schema (1024d) is model-agnostic — switching models requires re-embedding but no collection schema changes.

---

## 9. Revised Spec Wording

The design doc currently reads:
> "Embedding: e5-large-v2 (self-hosted) or text-embedding-3-large (OpenAI) — Self-hosted = no per-call cost at scale"

Suggested update:
> "Embedding: text-embedding-3-large via OpenAI API (Phase 1–4), self-hosted in-cluster (Phase 4+). Cutover trigger: 30K users OR P95 embedding latency > 100ms. During Phase 3, deploy shadow self-hosted infra for quality/latency validation before cutover."

---

## Appendix A: Implementation Notes for Phase 2

```python
# Ingestion Service — embedding call
from openai import AsyncOpenAI

client = AsyncOpenAI()

async def embed_chunks(texts: list[str]) -> list[list[float]]:
    """Embed a batch of chunks. Returns 1024-dim vectors."""
    response = await client.embeddings.create(
        model="text-embedding-3-large",
        input=texts,
        dimensions=1024,  # Critical: reduces storage/memory 3x vs 3072d
        encoding_format="float",
    )
    return [item.embedding for item in response.data]

# Recommended batch size: 100-500 texts per API call
# OpenAI max: 2048 inputs per request, 8191 tokens per input
```

```python
# Search Service — with Redis caching
import hashlib, json
from redis.asyncio import Redis

async def embed_query(query: str, course_id: str, redis: Redis) -> list[float]:
    cache_key = f"emb:{hashlib.sha256(f'{query}:{course_id}'.encode()).hexdigest()[:16]}"

    cached = await redis.get(cache_key)
    if cached:
        return json.loads(cached)

    embedding = (await embed_chunks([query]))[0]
    await redis.setex(cache_key, 21600, json.dumps(embedding))  # 6h TTL
    return embedding
```

---

## Appendix B: Self-Hosted Deployment (Phase 3 Shadow)

When ready to deploy shadow infrastructure:

```yaml
# text-embeddings-inference deployment (Helm values sketch)
image: ghcr.io/huggingface/text-embeddings-inference:1.5
model: BAAI/bge-large-en-v1.5  # Re-evaluate at Phase 3 — pick MTEB leader
resources:
  requests:
    nvidia.com/gpu: "1"
    memory: "8Gi"
  limits:
    nvidia.com/gpu: "1"
    memory: "12Gi"
nodeSelector:
  cloud.google.com/gke-nodepool: gpu-pool
env:
  - name: MAX_CONCURRENT_REQUESTS
    value: "512"
  - name: MAX_BATCH_TOKENS
    value: "16384"
```

Throughput on NVIDIA L4: ~2,000–4,000 embeddings/sec at batch sizes of 32–64. Single replica handles TeachersLounge workload up to ~100K DAU.
