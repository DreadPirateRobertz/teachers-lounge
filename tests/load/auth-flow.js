/**
 * k6 load test — critical path: authentication flow.
 *
 * Path: POST /auth/login → GET /gaming/profile → GET /quests/daily
 * Target: 1000 concurrent users, p99 <500ms per request.
 *
 * Usage:
 *   k6 run tests/load/auth-flow.js \
 *     --env BASE_URL=https://api-staging.teacherslounge.app
 */
import http from "k6/http";
import { check, group, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

// Custom metrics for per-step visibility
const loginTTFB = new Trend("login_ttfb_ms");
const profileTTFB = new Trend("profile_ttfb_ms");
const questsTTFB = new Trend("quests_ttfb_ms");
const authErrorRate = new Rate("auth_errors");

export const options = {
  stages: [
    { duration: "2m", target: 200 },   // ramp to 200 VUs
    { duration: "5m", target: 500 },   // ramp to 500 VUs
    { duration: "3m", target: 1000 },  // ramp to 1000 VUs (peak)
    { duration: "5m", target: 1000 },  // sustain peak
    { duration: "2m", target: 0 },     // cool down
  ],
  thresholds: {
    // Primary SLO: p99 <500ms for each hop
    login_ttfb_ms: ["p(99)<500"],
    profile_ttfb_ms: ["p(99)<500"],
    quests_ttfb_ms: ["p(99)<500"],
    // Overall HTTP error rate
    auth_errors: ["rate<0.01"],
    http_req_failed: ["rate<0.01"],
  },
};

/** Unique per-VU test credentials — load-test accounts must exist in staging. */
function credentials() {
  return {
    email: `loadtest+${__VU}@teacherslounge.app`,
    password: "LoadTest123!",
  };
}

export default function () {
  let token = null;

  group("login", () => {
    const res = http.post(
      `${BASE_URL}/api/v1/auth/login`,
      JSON.stringify(credentials()),
      { headers: { "Content-Type": "application/json" } }
    );
    loginTTFB.add(res.timings.waiting);

    const ok = check(res, {
      "login status 200": (r) => r.status === 200,
      "login returns token": (r) => {
        try {
          return !!JSON.parse(r.body).token;
        } catch {
          return false;
        }
      },
    });
    authErrorRate.add(!ok);

    if (res.status === 200) {
      token = JSON.parse(res.body).token;
    }
  });

  if (!token) {
    sleep(1);
    return;
  }

  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };

  group("gaming_profile", () => {
    const res = http.get(`${BASE_URL}/api/v1/gaming/profile`, { headers });
    profileTTFB.add(res.timings.waiting);
    const ok = check(res, { "profile 200": (r) => r.status === 200 });
    authErrorRate.add(!ok);
  });

  group("daily_quests", () => {
    const res = http.get(`${BASE_URL}/api/v1/quests/daily`, { headers });
    questsTTFB.add(res.timings.waiting);
    const ok = check(res, { "quests 200": (r) => r.status === 200 });
    authErrorRate.add(!ok);
  });

  // Realistic think-time between requests
  sleep(Math.random() * 2 + 0.5);
}
