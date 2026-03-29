# Qdrant Deployment Design — TeachersLounge Phase 2

**Date**: 2026-03-29
**Status**: Design — Ready for Review
**Author**: dink (crew, teachers_lounge)
**Feeds into**: tl-sui (Ingestion Service implementation)

---

## Overview

Qdrant serves as the primary vector database for TeachersLounge's RAG pipeline. It holds curriculum chunks, concept embeddings, and aggregated cross-student insights. This document covers the Phase 2 deployment configuration: 3-replica distributed cluster on GKE, collection schema with HNSW + scalar quantization, payload indexing, GCS snapshots, and resource sizing.

Helm values implementing this design: `infra/helm/qdrant/values.yaml`

---

## 1. Cluster Architecture: 3-Replica Distributed Deployment

### Topology

```
┌─────────────────────────────────────────────────────┐
│  GKE Cluster — general-pool (e2-standard-4 nodes)   │
│                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐  │
│  │  qdrant-0    │  │  qdrant-1    │  │ qdrant-2 │  │
│  │  (node A)    │  │  (node B)    │  │ (node C) │  │
│  │              │  │              │  │          │  │
│  │  HTTP :6333  │  │  HTTP :6333  │  │ HTTP:6333│  │
│  │  gRPC :6334  │  │  gRPC :6334  │  │ gRPC:6334│  │
│  │  P2P  :6335  │◄─►  P2P  :6335  │◄─►P2P :6335│  │
│  └──────────────┘  └──────────────┘  └──────────┘  │
│         │                  │               │        │
│  ┌──────┴───┐       ┌──────┴──┐    ┌───────┴──┐    │
│  │ PVC 50Gi │       │ PVC 50Gi│    │ PVC 50Gi │    │
│  └──────────┘       └─────────┘    └──────────┘    │
└─────────────────────────────────────────────────────┘
                          │
                   ClusterIP Service
                   (internal access only)
                          │
              ┌───────────┴─────────────┐
              │  Ingestion Service       │
              │  Search Service          │
              └─────────────────────────┘
```

### Why 3 Replicas

- **High availability**: tolerate loss of 1 replica without downtime (PDB enforces minAvailable: 2)
- **Replication factor = 2**: each shard is stored on 2 of 3 nodes, so any single node failure is non-destructive
- **Raft consensus**: leader election via built-in consensus; no Zookeeper/etcd dependency
- **Write quorum**: writes succeed when acknowledged by quorum (2/3 nodes)

### Shard Configuration (per collection)

For Phase 2 data volumes, use conservative sharding:
- `shard_number: 2` — 2 shards per collection
- `replication_factor: 2` — each shard replicated to 2 nodes
- Result: 4 shard replicas distributed across 3 nodes

At Phase 3 (>1M chunks), increase to `shard_number: 6` and reindex.

---

## 2. Collection Designs

### 2.1 `curriculum` — Main Knowledge Base

Purpose: Stores all course material chunks with hybrid search support (dense + sparse).

```python
from qdrant_client import QdrantClient
from qdrant_client.models import (
    VectorParams, Distance, SparseVectorParams, SparseIndexParams,
    HnswConfigDiff, ScalarQuantizationConfig, ScalarType,
    QuantizationConfig, ScalarQuantization,
)

client.create_collection(
    collection_name="curriculum",
    vectors_config={
        # Dense: e5-large-v2 or OpenAI text-embedding-3-large (1024d)
        "dense": VectorParams(
            size=1024,
            distance=Distance.COSINE,
            hnsw_config=HnswConfigDiff(
                m=16,
                ef_construct=100,
                full_scan_threshold=10_000,
                on_disk=False,      # HNSW graph in RAM — required for <50ms retrieval
            ),
            quantization_config=QuantizationConfig(
                scalar=ScalarQuantization(
                    type=ScalarType.INT8,
                    quantile=0.99,   # Clip outliers — improves quantization quality
                    always_ram=True, # Keep quantized vectors in RAM
                )
            ),
            on_disk=False,          # Original float32 vectors on disk, quantized in RAM
        ),
    },
    sparse_vectors_config={
        # Sparse: BM25 token weights (computed by Ingestion Service)
        "sparse": SparseVectorParams(
            index=SparseIndexParams(
                on_disk=False,      # Sparse index in RAM for fast keyword retrieval
            ),
        ),
    },
    shard_number=2,
    replication_factor=2,
    write_consistency_factor=1,
)
```

**Payload schema** (stored per point):

| Field | Type | Example | Purpose |
|-------|------|---------|---------|
| `course_id` | UUID string | `"abc-123"` | Primary filter — every query filters on this |
| `material_id` | UUID string | `"def-456"` | Source document reference |
| `chunk_id` | UUID string | `"ghi-789"` | Links to Postgres `chunks` table |
| `chapter` | string | `"Chapter 3"` | Hierarchical path component |
| `section` | string | `"3.2 Quadratic Formula"` | Section heading |
| `page` | int | `42` | Source page number |
| `content_type` | string enum | `"text"`, `"table"`, `"equation"`, `"figure"`, `"quiz"` | Enables type-filtered retrieval |
| `hierarchical_path` | string | `"Chapter 3 > Section 3.2 > Quadratic Formula"` | Prepended to chunk for context |

### 2.2 `concepts` — Knowledge Graph Node Embeddings

Purpose: Stores concept embeddings for prerequisite-aware retrieval and concept gap detection.

```python
client.create_collection(
    collection_name="concepts",
    vectors_config={
        "dense": VectorParams(
            size=1024,
            distance=Distance.COSINE,
            hnsw_config=HnswConfigDiff(
                m=16,
                ef_construct=100,
                full_scan_threshold=10_000,
            ),
            quantization_config=QuantizationConfig(
                scalar=ScalarQuantization(
                    type=ScalarType.INT8,
                    quantile=0.99,
                    always_ram=True,
                )
            ),
        ),
    },
    shard_number=1,          # Smaller collection, single shard sufficient
    replication_factor=2,
)
```

**Payload schema**:

| Field | Type | Purpose |
|-------|------|---------|
| `concept_name` | string | Human-readable concept name |
| `course_id` | UUID string | Scopes concept to a course |
| `difficulty` | float [0,1] | Normalized difficulty score |
| `prerequisite_ids` | UUID[] | Concept IDs this concept depends on |
| `concept_id` | UUID string | Links to Postgres `concepts` table |

### 2.3 `insights` — Cross-Student Aggregate Intelligence

Purpose: Stores nightly-generated aggregate insights from the Analytics Service (anonymized).

```python
client.create_collection(
    collection_name="insights",
    vectors_config={
        "dense": VectorParams(
            size=1024,
            distance=Distance.COSINE,
            hnsw_config=HnswConfigDiff(
                m=16,
                ef_construct=100,
                full_scan_threshold=10_000,
            ),
            quantization_config=QuantizationConfig(
                scalar=ScalarQuantization(
                    type=ScalarType.INT8,
                    quantile=0.99,
                    always_ram=True,
                )
            ),
        ),
    },
    shard_number=1,
    replication_factor=2,
)
```

**Payload schema**:

| Field | Type | Purpose |
|-------|------|---------|
| `topic` | string | Topic this insight covers |
| `insight_type` | string enum | `"misconception"`, `"difficulty_spike"`, `"effective_explanation"` |
| `confidence` | float [0,1] | Statistical confidence of insight |
| `generated_at` | ISO datetime | Nightly batch timestamp |
| `student_count` | int | Anonymized population size (min 10 for k-anonymity) |
| `insight_text` | string | The insight content (no PII) |

---

## 3. HNSW Index Parameters

HNSW (Hierarchical Navigable Small World) is Qdrant's primary ANN index. These parameters balance recall quality vs memory/build time:

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| `m` | 16 | Connections per node in the HNSW graph. 16 is standard; higher = better recall, more memory. Doubling to 32 improves recall ~1% but doubles memory. |
| `ef_construct` | 100 | Beam width during index construction. Higher = better index quality, slower build. 100 is safe default; drop to 64 if ingestion is too slow. |
| `full_scan_threshold` | 10,000 | Collections with fewer vectors than this use brute-force (more accurate). Important for early Phase 2 when collections are small. |
| `on_disk` | false | HNSW graph stored in RAM. Required for <50ms retrieval. At scale (Phase 5+), `on_disk: true` trades latency for memory. |

### Scalar Quantization (int8) — 4× Memory Reduction

Float32 vectors (1024d × 4 bytes = 4KB/vector) → INT8 (1024d × 1 byte = 1KB/vector).

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| `type` | `INT8` | 8-bit integer quantization — best quality/compression trade-off |
| `quantile` | 0.99 | Clips top/bottom 0.5% of values before quantizing, preserving the bulk of the distribution |
| `always_ram` | true | Quantized vectors always kept in RAM; original float32 on disk (loaded on re-ranking) |

**Recall impact**: INT8 quantization causes ~0.5–2% recall degradation vs full float32, acceptable for educational RAG where exact recall is less critical than MTEB absolute score.

**Memory budget estimate** (Phase 2: 500K chunks in `curriculum`):

| Layer | Per-Vector | 500K Chunks | Notes |
|-------|-----------|-------------|-------|
| HNSW graph | ~400B | ~200MB | In RAM always |
| INT8 quantized vectors | 1KB | ~1GB | In RAM always |
| Float32 vectors | 4KB | ~4GB | On disk; loaded for re-ranking if configured |
| Payload store | ~500B | ~250MB | On disk |
| **RAM footprint** | | **~1.2GB** | Per replica (HNSW + INT8) |

**At 3 replicas**: ~3.6GB RAM for vector data, leaving ~6.4GB for OS, Qdrant runtime, and WAL on 10Gi limit.

---

## 4. Payload Index: `course_id`

Every query to `curriculum` filters on `course_id` (students can only search their own courses). Without a payload index, Qdrant performs a full scan of filtered payload before ANN search.

```python
client.create_payload_index(
    collection_name="curriculum",
    field_name="course_id",
    field_schema=PayloadSchemaType.KEYWORD,    # UUID stored as keyword
)
```

**Also index** for Search Service filtering flexibility:

```python
# Enable content-type filtered retrieval
client.create_payload_index(
    collection_name="curriculum",
    field_name="content_type",
    field_schema=PayloadSchemaType.KEYWORD,
)

# Enable chapter-scoped searches
client.create_payload_index(
    collection_name="curriculum",
    field_name="chapter",
    field_schema=PayloadSchemaType.KEYWORD,
)
```

These indexes are created by the Ingestion Service init job on first deployment. They are stored in a dedicated index structure and consume negligible memory.

---

## 5. GCS Snapshot Schedule

### Design

Snapshots are taken nightly via a Kubernetes CronJob. Each snapshot is a consistent point-in-time copy of all vector data, uploaded to `gs://tvtutor-backups/qdrant-snapshots/`.

### CronJob Flow

```
02:00 UTC — CronJob fires
    │
    ▼
POST /collections/{name}/snapshots on each collection
    → Qdrant writes snapshot to /qdrant/snapshots/ (PVC)
    │
    ▼
gsutil -m cp /qdrant/snapshots/*.snapshot \
    gs://tvtutor-backups/qdrant-snapshots/$(date +%Y-%m-%d)/
    │
    ▼
DELETE /collections/{name}/snapshots/{snapshot_name}
    → Clean up local snapshot file
```

### Retention

- **Local**: Snapshots deleted immediately after upload (disk space management)
- **GCS**: 14-day retention via GCS lifecycle policy on `tvtutor-backups/qdrant-snapshots/`
- **Coldline transition**: Snapshots older than 90 days move to Coldline storage

### Restore Procedure

```bash
# 1. Download snapshot from GCS
gsutil cp gs://tvtutor-backups/qdrant-snapshots/2026-03-28/curriculum_*.snapshot .

# 2. Upload to Qdrant (recreates collection from snapshot)
curl -X POST "http://qdrant-svc:6333/collections/curriculum/snapshots/upload" \
  -H "api-key: ${QDRANT_API_KEY}" \
  -F "snapshot=@curriculum_2026-03-28.snapshot"

# 3. Verify collection health
curl "http://qdrant-svc:6333/collections/curriculum" -H "api-key: ${QDRANT_API_KEY}"
```

### Workload Identity Setup (required before deploy)

```bash
# Create GSA for snapshot job
gcloud iam service-accounts create qdrant-snapshot-sa \
  --project=tvtutor-prod

# Grant GCS write permission
gcloud storage buckets add-iam-policy-binding gs://tvtutor-backups \
  --member="serviceAccount:qdrant-snapshot-sa@tvtutor-prod.iam.gserviceaccount.com" \
  --role="roles/storage.objectCreator"

# Bind KSA to GSA via Workload Identity
gcloud iam service-accounts add-iam-policy-binding \
  qdrant-snapshot-sa@tvtutor-prod.iam.gserviceaccount.com \
  --role="roles/iam.workloadIdentityUser" \
  --member="serviceAccount:tvtutor-prod.svc.id.goog[qdrant/qdrant-snapshot-sa]"
```

---

## 6. Persistence: GKE PVC Configuration

Each StatefulSet replica gets its own PVC (standard GKE StatefulSet behavior):

| Volume | Size | Storage Class | Mount Path | Purpose |
|--------|------|--------------|------------|---------|
| storage | 50Gi | `standard-rwo` | `/qdrant/storage` | Vector data, HNSW index, payloads |
| snapshots | 20Gi | `standard-rwo` | `/qdrant/snapshots` | Transient snapshot staging before GCS upload |

**Storage class notes**:
- `standard-rwo` = Google Balanced Persistent Disk (pd-balanced). Read-Write-Once. Suitable for Phase 2.
- If IOPS become a bottleneck (symptom: slow index rebuild after restart), upgrade to `premium-rwo` (pd-ssd). Cost: ~3× more expensive.
- `ReadWriteOnce` is correct for StatefulSet — each pod gets its own disk.

**Capacity planning**:

| Phase | Users | Chunks (curriculum) | Float32 (disk) | INT8 (RAM) | PVC Needed |
|-------|-------|--------------------|-----------------|-----------|-----------:|
| Phase 2 | 1K | 500K | 2GB | 500MB | 10Gi |
| Phase 3 | 10K | 5M | 20GB | 5GB | 30Gi |
| Phase 4 | 50K | 25M | 100GB | 25GB | 150Gi |
| Phase 5 | 100K | 50M | 200GB | 50GB | 300Gi |

50Gi PVC for Phase 2 provides 5× headroom. PVCs can be expanded in-place on GKE without downtime (CSI volume expansion).

---

## 7. Resource Requests and Limits

### Sizing Rationale

General-pool node: e2-standard-4 (4 vCPU, 16GB RAM). With 3 Qdrant pods running on 3 separate nodes:

| Resource | Request | Limit | Rationale |
|----------|---------|-------|-----------|
| CPU | 1500m (1.5 cores) | 3000m (3 cores) | Qdrant is I/O bound at rest; CPU spikes during index rebuild and replication |
| Memory | 4Gi | 10Gi | 1.2GB vectors in RAM + Qdrant runtime (~500MB) + WAL + headroom; OOM on rebuild if limit too low |

**Node capacity**: e2-standard-4 has 4 vCPU and 16GB. After Qdrant (1.5 CPU / 4Gi request), the remaining 2.5 CPU / 12GB is available for node daemonsets (Fluentbit, kube-proxy, Prometheus node exporter, ~1.5 CPU / 2GB) plus other co-scheduled pods.

**Recommended**: dedicate 1 general-pool node per Qdrant replica by setting node affinity to spread replicas, or use the `topologySpreadConstraints` in the Helm values. Do not co-schedule Ingestion Service workers on the same nodes as Qdrant — they have heavy CPU bursts during OCR/transcription.

### Vertical Scaling Triggers

Monitor these metrics (Prometheus + Grafana):

| Metric | Alert Threshold | Action |
|--------|----------------|--------|
| `qdrant_memory_active_bytes` / limit | > 80% | Increase memory limit |
| `qdrant_collections_total_vector_count` | > 2M per shard | Add shards (re-shard collection) |
| Vector search p95 latency | > 100ms | Investigate: index fragmentation or resource pressure |
| Disk used on storage PVC | > 70% | Expand PVC via storage class resize |

---

## 8. API Key & Network Security

- Qdrant API key stored in Kubernetes Secret `qdrant-api-key` (namespace: `qdrant`)
- All client connections must include `api-key` header
- Service is `ClusterIP` only — never exposed via LoadBalancer or Ingress
- Ingestion Service and Search Service access Qdrant via internal DNS: `qdrant.qdrant.svc.cluster.local:6333`
- Consider adding mTLS via Istio in Phase 3 (service mesh already in architecture spec)

---

## 9. Phase 2 Deployment Checklist

- [ ] `qdrant` namespace created
- [ ] `qdrant-api-key` secret created
- [ ] Helm repo added: `helm repo add qdrant https://qdrant.github.io/qdrant-helm`
- [ ] GKE gpu-pool NOT required for Qdrant (general-pool only)
- [ ] Workload Identity configured for `qdrant-snapshot-sa`
- [ ] GCS bucket `tvtutor-backups` exists with lifecycle policy
- [ ] StorageClass `standard-rwo` available in cluster
- [ ] Prometheus-operator CRDs installed (for ServiceMonitor)
- [ ] Collections created via init job after first deploy (curriculum, concepts, insights)
- [ ] Payload indexes created: `course_id`, `content_type`, `chapter` on curriculum
- [ ] Snapshot CronJob verified with manual trigger before leaving Phase 2

---

## 10. Open Questions / Future Work

| Item | Phase | Notes |
|------|-------|-------|
| `diagrams` collection (CLIP, 768d) | Phase 6 | Multi-modal image search — separate collection, different model |
| Shard count increase | Phase 3+ | Re-shard at >2M chunks; requires collection recreation |
| Switch HNSW `on_disk: true` | Phase 5+ | Trade latency for memory at very high vector counts |
| Qdrant Cloud vs self-hosted | Phase 6+ | Revisit ops burden trade-off at scale |
| Dedicated Qdrant node pool | Phase 3 | Move off general-pool to memory-optimized nodes (n2-highmem-4) |
