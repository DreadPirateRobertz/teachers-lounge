# Qdrant HNSW Parameter Benchmark (tl-8be)

## Methodology

- Corpus: 10000 synthetic unit vectors, dim=1536
- Queries: 200 synthetic unit vectors (disjoint RNG seed)
- Ground truth: exact cosine top-10 via brute-force matrix multiply
- Qdrant: 1.9.7 @ http://127.0.0.1:6333
- Distance: COSINE; no scalar quantization
- Seed: 0

Synthetic unit vectors approximate the curriculum collection's
embedding distribution. Absolute recall numbers under real
text-embedding-3-small output will differ; **relative rankings
across (m, ef_construct) are the signal to trust**.

## Results

| m | ef_construct | recall@10 | p50 latency (ms) | p99 latency (ms) |
|---|---|---|---|---|
| 8 | 64 | 1.000 | 3.06 | 4.12 |
| 8 | 100 | 1.000 | 3.10 | 4.19 |
| 8 | 200 | 1.000 | 3.14 | 5.04 |
| 16 | 64 | 1.000 | 3.11 | 4.67 |
| 16 | 100 | 1.000 | 3.04 | 4.47 |
| 16 | 200 | 1.000 | 3.06 | 5.16 |
| 32 | 64 | 1.000 | 5.86 | 12.32 |
| 32 | 100 | 1.000 | 6.80 | 15.64 |
| 32 | 200 | 1.000 | 6.59 | 18.00 |

## Summary

- Highest recall@10: **m=8, ef_construct=64** (recall 1.000, p99 4.12 ms)
- Fastest p99: **m=8, ef_construct=64** (recall 1.000, p99 4.12 ms)

> **Recall caveat.** All configurations returned recall@10 within
> 0.000 of each other. Random unit vectors
> in 1536 dimensions are near-orthogonal, so any HNSW graph finds
> the exact top-k and recall cannot discriminate parameters on
> this corpus. **Only the latency column is informative here.**
> Before changing production defaults, re-run against real
> curriculum embeddings — real text embeddings cluster and will
> expose recall regressions that synthetic data hides.

## Recommendation

Current production defaults (`infra/helm/qdrant/values.yaml`) are
`m=16, ef_construct=100`. Based on this synthetic sweep:

- `m=32` shows a clear ~2x p99 latency penalty vs `m=8`/`m=16` —
  a red flag for any future tuning push toward higher `m`.
- `m=8` and `m=16` are within noise on latency; with real embeddings,
  `m=16` is expected to retain higher recall in dense clusters.
- **No config change proposed from this run.** Re-run against real
  curriculum embeddings (≥50k chunks) before considering a tweak.
