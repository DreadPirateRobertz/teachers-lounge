# Qdrant Collection Payload Schema

**Collection**: `curriculum`  
**Refs**: `infra/helm/qdrant/values.yaml`, `docs/qdrant-design.md`

This document is the contract between the Ingestion Service (tl-ysp) and the
Search Service (tl-12c / tl-p99). Any field listed here must be present on
every point stored in the collection.

---

## Vector Fields

| Field name | Type     | Dimension | Notes |
|------------|----------|-----------|-------|
| `dense`    | float32  | 1536      | OpenAI `text-embedding-3-small` (Phase 2). Self-hosted TEI Phase 4+. |
| `sparse`   | sparse   | 30000     | BM25 TF weights. Indexed at ingestion via `_tokenize()` in qdrant.py. |

---

## Payload Fields

| Field          | Type   | Required | Notes |
|----------------|--------|----------|-------|
| `chunk_id`     | UUID   | yes      | Stable ID for the chunk; survives re-indexing |
| `course_id`    | UUID   | yes      | FERPA scope key. Always indexed (`keyword`). Search filters on this. |
| `material_id`  | UUID   | yes      | Parent material (row in `materials` table) |
| `content`      | string | yes      | Raw text of the chunk |
| `content_type` | string | yes      | `text` \| `table` \| `figure` \| `code` |
| `page`         | int    | no       | 1-based page number (PDF/Office only) |
| `chapter`      | string | no       | Chapter heading, if available |
| `section`      | string | no       | Section heading, if available |

---

## Payload Indexes

| Field          | Index type | Reason |
|----------------|------------|--------|
| `course_id`    | `keyword`  | Every search query filters on course_id. |
| `content_type` | `keyword`  | Allows filtering by content type (future). |
| `chapter`      | `keyword`  | Allows chapter-scoped search (future). |

Indexes are created at collection initialization time (see `docs/qdrant-design.md § 4`).

---

## Example Point

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "payload": {
    "chunk_id": "550e8400-e29b-41d4-a716-446655440000",
    "course_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "material_id": "deadbeef-0000-0000-0000-000000000001",
    "content": "Entropy is a measure of disorder in a thermodynamic system...",
    "content_type": "text",
    "page": 42,
    "chapter": "Chapter 3: Thermodynamics",
    "section": "3.2 Entropy"
  },
  "vector": {
    "dense": [0.012, -0.034, ...],
    "sparse": {"indices": [1234, 5678, 9012], "values": [0.45, 0.30, 0.25]}
  }
}
```

---

## Ingestion Contract

The Ingestion Service (`services/ingestion`) must:

1. Compute `dense` embeddings via `text-embedding-3-small` (1536-dim) at chunk upload time.
2. Compute `sparse` TF weights using the same `_tokenize()` logic in
   `services/search/app/services/qdrant.py` (or a shared utility once extracted).
3. Upsert points with all **required** payload fields present.
4. Use the `chunk_id` UUID as the Qdrant point ID.

The Search Service will fail with payload extraction errors if required fields
are absent.
