/**
 * k6 load test — search service: hybrid vector search + diagram search.
 *
 * Endpoints exercised:
 *   GET /v1/search?q=...&course_id=...
 *   GET /v1/diagrams?q=...&course_id=...  (if available)
 *
 * Targets (Phase 8 exit criteria):
 *   - search p95 <2s (embedding + dual Qdrant + rerank pipeline)
 *
 * k6 notes:
 *   - No auth required on search in current implementation (tl-sui milestone pending).
 *   - COURSE_ID must exist in staging Qdrant collection.
 *
 * Usage:
 *   k6 run tests/load/search-service.js \
 *     --env BASE_URL=https://api-staging.teacherslounge.app \
 *     --env COURSE_ID=<uuid>
 */
import http from "k6/http";
import { check, group, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8001";
const COURSE_ID = __ENV.COURSE_ID || "00000000-0000-0000-0000-000000000001";

const searchTTFB = new Trend("search_ttfb_ms", true);
const diagramSearchTTFB = new Trend("diagram_search_ttfb_ms", true);
const searchErrorRate = new Rate("search_errors");

// Representative student queries from different subjects
const QUERIES = [
  "mitochondria ATP production",
  "quadratic formula derivation",
  "photosynthesis vs cellular respiration",
  "Newton second law examples",
  "French Revolution causes",
  "DNA replication semiconservative",
  "integration by parts",
  "supply demand equilibrium",
  "Civil War reconstruction era",
  "chemical equilibrium Le Chatelier",
];

export const options = {
  stages: [
    { duration: "2m", target: 50 },   // ramp to 50 VUs
    { duration: "5m", target: 200 },  // ramp to peak (search is CPU-bound on embedding)
    { duration: "8m", target: 200 },  // sustain peak
    { duration: "2m", target: 50 },   // taper
    { duration: "1m", target: 150 },  // spike
    { duration: "3m", target: 150 },  // sustain spike
    { duration: "2m", target: 0 },    // cool down
  ],
  thresholds: {
    // Phase 8 exit criteria: p95 <2s for embedding + Qdrant + rerank pipeline
    search_ttfb_ms: ["p(95)<2000", "p(99)<4000"],
    diagram_search_ttfb_ms: ["p(95)<2000", "p(99)<4000"],
    search_errors: ["rate<0.02"],
    http_req_failed: ["rate<0.02"],
  },
};

export default function () {
  const query = QUERIES[(__VU + __ITER) % QUERIES.length];

  group("text_search", () => {
    const res = http.get(
      `${BASE_URL}/v1/search?q=${encodeURIComponent(query)}&course_id=${COURSE_ID}&limit=10`,
      { timeout: "15s" }
    );
    searchTTFB.add(res.timings.waiting);

    const ok = check(res, {
      "search 200": (r) => r.status === 200,
      "search returns results": (r) => {
        try {
          const body = JSON.parse(r.body);
          return Array.isArray(body.results);
        } catch {
          return false;
        }
      },
      "search_mode present": (r) => {
        try {
          return !!JSON.parse(r.body).search_mode;
        } catch {
          return false;
        }
      },
    });
    searchErrorRate.add(!ok);
  });

  // Diagram search — visual learner path
  group("diagram_search", () => {
    const res = http.get(
      `${BASE_URL}/v1/diagrams?q=${encodeURIComponent(query)}&course_id=${COURSE_ID}&limit=5`,
      { timeout: "15s" }
    );
    diagramSearchTTFB.add(res.timings.waiting);

    // Diagram endpoint may not exist in all envs — accept 404
    const ok = check(res, {
      "diagram search 200 or 404": (r) => r.status === 200 || r.status === 404,
    });
    searchErrorRate.add(!ok);
  });

  // Simulate student reading search results before clicking a link
  sleep(Math.random() * 3 + 1);
}
