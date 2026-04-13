/**
 * k6 load test — ingestion service: file upload + status polling.
 *
 * Path: POST /v1/ingest/upload (multipart) → GET /v1/ingest/{id}/status
 * Targets:
 *   - upload p95 <5s (TTFB budget from Phase 8 exit criteria)
 *   - status poll p95 <200ms
 *
 * Notes:
 *   - Upload endpoint requires multipart/form-data with a 'file' field and 'course_id' query param.
 *   - k6 FormData simulates a small PDF payload (1KB synthetic bytes) to exercise the pipeline
 *     without saturating network — true large-file tests use the soak profile.
 *   - Status polling is fire-and-forget: we poll once per VU iteration for simplicity.
 *
 * Usage:
 *   k6 run tests/load/ingestion-service.js \
 *     --env BASE_URL=https://api-staging.teacherslounge.app \
 *     --env AUTH_TOKEN=<staging-service-token> \
 *     --env COURSE_ID=<uuid>
 */
import http from "k6/http";
import { check, group, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";
import { uuidv4 } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8082";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "test-token";
const COURSE_ID = __ENV.COURSE_ID || "00000000-0000-0000-0000-000000000001";

const uploadTTFB = new Trend("upload_ttfb_ms", true);
const statusPollDuration = new Trend("status_poll_ms");
const uploadErrorRate = new Rate("upload_errors");

// Synthetic 1 KB payload (simulates a tiny PDF fragment)
const FAKE_BYTES = new Uint8Array(1024).fill(0x25).buffer;  // 0x25 = '%' (valid PDF magic prefix)

export const options = {
  stages: [
    { duration: "2m", target: 20 },   // ramp to 20 VUs (upload is heavy — keep VUs low)
    { duration: "5m", target: 50 },   // ramp to 50 VUs
    { duration: "3m", target: 50 },   // sustain
    { duration: "2m", target: 100 },  // spike
    { duration: "3m", target: 100 },  // sustain spike
    { duration: "2m", target: 0 },    // cool down
  ],
  thresholds: {
    // Phase 8 exit criteria: upload p95 <5s
    upload_ttfb_ms: ["p(95)<5000", "p(99)<10000"],
    status_poll_ms: ["p(95)<200", "p(99)<500"],
    upload_errors: ["rate<0.02"],
    http_req_failed: ["rate<0.02"],
  },
};

export default function () {
  const headers = {
    Authorization: `Bearer ${AUTH_TOKEN}`,
  };

  let materialId = null;

  group("upload_file", () => {
    const formData = {
      file: http.file(FAKE_BYTES, `test-${uuidv4()}.pdf`, "application/pdf"),
    };

    const res = http.post(
      `${BASE_URL}/v1/ingest/upload?course_id=${COURSE_ID}`,
      formData,
      { headers, timeout: "30s" }
    );
    uploadTTFB.add(res.timings.waiting);

    const ok = check(res, {
      "upload 202": (r) => r.status === 202,
      "upload returns material_id": (r) => {
        try {
          return !!JSON.parse(r.body).material_id;
        } catch {
          return false;
        }
      },
    });
    uploadErrorRate.add(!ok);

    if (res.status === 202) {
      try {
        materialId = JSON.parse(res.body).material_id;
      } catch {
        // continue without status poll
      }
    }
  });

  if (!materialId) {
    sleep(2);
    return;
  }

  // Poll status once per iteration (real clients poll until complete)
  group("poll_status", () => {
    const res = http.get(
      `${BASE_URL}/v1/ingest/${materialId}/status`,
      { headers }
    );
    statusPollDuration.add(res.timings.duration);

    const ok = check(res, {
      "status 200": (r) => r.status === 200,
      "status has processing_status": (r) => {
        try {
          return !!JSON.parse(r.body).processing_status;
        } catch {
          return false;
        }
      },
    });
    uploadErrorRate.add(!ok);
  });

  // Realistic think-time: user waits after uploading material
  sleep(Math.random() * 3 + 2);
}
