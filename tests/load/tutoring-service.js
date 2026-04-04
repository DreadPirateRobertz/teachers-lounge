import http from "k6/http";
import { check, sleep } from "k6";
import { standardStages, streamingThresholds } from "./k6-config.js";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8000";
const TOKEN = __ENV.AUTH_TOKEN || "test-token";

export const options = {
  stages: standardStages,
  thresholds: streamingThresholds,
};

export default function () {
  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${TOKEN}`,
  };

  // Health check
  const health = http.get(`${BASE_URL}/health`);
  check(health, { "health 200": (r) => r.status === 200 });

  // Chat message (SSE streaming endpoint)
  const chatRes = http.post(
    `${BASE_URL}/api/v1/chat`,
    JSON.stringify({
      message: "What is photosynthesis?",
      session_id: `load-${__VU}-${__ITER}`,
    }),
    { headers, timeout: "30s" }
  );
  check(chatRes, {
    "chat 200 or 401": (r) => r.status === 200 || r.status === 401,
  });

  // Search within tutoring context
  const search = http.get(
    `${BASE_URL}/api/v1/search?q=mitochondria&limit=5`,
    { headers }
  );
  check(search, {
    "search 200 or 401": (r) => r.status === 200 || r.status === 401,
  });

  sleep(3);
}
