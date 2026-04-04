import http from "k6/http";
import { check, sleep } from "k6";
import { standardStages, standardThresholds } from "./k6-config.js";

const BASE_URL = __ENV.BASE_URL || "http://localhost:3000";

export const options = {
  stages: standardStages,
  thresholds: {
    ...standardThresholds,
    http_req_duration: ["p(95)<800", "p(99)<2000"],
  },
};

export default function () {
  // Health endpoint
  const health = http.get(`${BASE_URL}/api/health`);
  check(health, { "health 200": (r) => r.status === 200 });

  // Landing page (SSR)
  const landing = http.get(`${BASE_URL}/`);
  check(landing, { "landing 200": (r) => r.status === 200 });

  // Dashboard page (SSR, auth-gated — expect redirect or 200)
  const dashboard = http.get(`${BASE_URL}/dashboard`);
  check(dashboard, {
    "dashboard 200 or 302": (r) => r.status === 200 || r.status === 302,
  });

  // Static asset (Next.js _next/static)
  const staticAsset = http.get(`${BASE_URL}/_next/static/chunks/webpack.js`);
  check(staticAsset, {
    "static 200 or 404": (r) => r.status === 200 || r.status === 404,
  });

  sleep(1);
}
