import http from "k6/http";
import { check, sleep } from "k6";
import { standardStages, standardThresholds } from "./k6-config.js";

const BASE_URL = __ENV.BASE_URL || "http://localhost:4000";
const API_KEY = __ENV.API_KEY || "sk-test-key";

export const options = {
  stages: standardStages,
  thresholds: standardThresholds,
};

export default function () {
  // Health check
  const health = http.get(`${BASE_URL}/health/readiness`);
  check(health, { "health 200": (r) => r.status === 200 });

  // Model list
  const models = http.get(`${BASE_URL}/v1/models`, {
    headers: { Authorization: `Bearer ${API_KEY}` },
  });
  check(models, { "models 200": (r) => r.status === 200 });

  // Chat completion (short prompt to test routing, not LLM latency)
  const chat = http.post(
    `${BASE_URL}/v1/chat/completions`,
    JSON.stringify({
      model: "gpt-4o-mini",
      messages: [{ role: "user", content: "ping" }],
      max_tokens: 5,
    }),
    {
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${API_KEY}`,
      },
      timeout: "30s",
    }
  );
  check(chat, {
    "chat 200 or 429": (r) => r.status === 200 || r.status === 429,
  });

  sleep(2);
}
