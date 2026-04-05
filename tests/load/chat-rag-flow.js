/**
 * k6 load test — critical path: chat / RAG streaming flow.
 *
 * Path: POST /tutoring/sessions/{id}/messages → SSE stream first token (TTFB).
 * Target: 200 concurrent sessions, p95 TTFB <2s.
 *
 * k6 measures TTFB via res.timings.waiting (time until first byte received).
 * For SSE responses that hold the connection open, this is the time to first chunk.
 *
 * Usage:
 *   k6 run tests/load/chat-rag-flow.js \
 *     --env BASE_URL=https://api-staging.teacherslounge.app \
 *     --env AUTH_TOKEN=<staging-service-token>
 */
import http from "k6/http";
import { check, group, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8000";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "test-token";

// Trend tracking TTFB for SSE stream (time-to-first-byte = first chunk latency)
const chatTTFB = new Trend("chat_ttfb_ms", true);
const sessionCreateDuration = new Trend("session_create_ms");
const chatErrorRate = new Rate("chat_errors");

// Representative student questions that exercise the RAG pipeline
const QUESTIONS = [
  "Explain the mitochondria and its role in ATP production.",
  "What is the quadratic formula and how do I derive it?",
  "How does photosynthesis differ from cellular respiration?",
  "What were the main causes of World War I?",
  "Explain Newton's second law with an example.",
];

export const options = {
  stages: [
    { duration: "2m", target: 50 },   // ramp to 50 sessions
    { duration: "3m", target: 200 },  // ramp to 200 sessions (peak)
    { duration: "8m", target: 200 },  // sustain peak load
    { duration: "2m", target: 0 },    // cool down
  ],
  thresholds: {
    // Primary SLO: p95 TTFB <2s for AI stream first token
    "chat_ttfb_ms": ["p(95)<2000", "p(99)<5000"],
    "session_create_ms": ["p(95)<500"],
    chat_errors: ["rate<0.02"],
    http_req_failed: ["rate<0.02"],
  },
};

export default function () {
  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${AUTH_TOKEN}`,
  };

  let sessionId = null;

  group("create_session", () => {
    const res = http.post(
      `${BASE_URL}/api/v1/sessions`,
      JSON.stringify({}),
      { headers }
    );
    sessionCreateDuration.add(res.timings.duration);

    const ok = check(res, {
      "session created 201": (r) => r.status === 201,
      "session has id": (r) => {
        try {
          return !!JSON.parse(r.body).session_id;
        } catch {
          return false;
        }
      },
    });
    chatErrorRate.add(!ok);

    if (res.status === 201) {
      sessionId = JSON.parse(res.body).session_id;
    }
  });

  if (!sessionId) {
    sleep(1);
    return;
  }

  group("send_message_sse", () => {
    const question = QUESTIONS[__VU % QUESTIONS.length];

    // POST to the chat endpoint — SSE stream.
    // k6's res.timings.waiting = TTFB (time until first byte/chunk).
    const res = http.post(
      `${BASE_URL}/api/v1/sessions/${sessionId}/messages`,
      JSON.stringify({ content: question }),
      {
        headers: {
          ...headers,
          Accept: "text/event-stream",
        },
        timeout: "30s",
        // k6 reads the full body — for SSE this means buffering the stream.
        // TTFB is still captured via timings.waiting.
      }
    );
    chatTTFB.add(res.timings.waiting);

    const ok = check(res, {
      "chat 200": (r) => r.status === 200,
      "response has content": (r) => r.body && r.body.length > 0,
    });
    chatErrorRate.add(!ok);
  });

  // Simulate student reading the response before sending next message
  sleep(Math.random() * 5 + 3);
}
