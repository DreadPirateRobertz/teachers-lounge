import http from "k6/http";
import { check, sleep } from "k6";
import { standardStages, standardThresholds } from "./k6-config.js";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export const options = {
  stages: standardStages,
  thresholds: standardThresholds,
};

export default function () {
  // Health check
  const health = http.get(`${BASE_URL}/health`);
  check(health, { "health 200": (r) => r.status === 200 });

  // Auth flow: login
  const loginRes = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({
      email: `loadtest+${__VU}@teacherslounge.app`,
      password: "LoadTest123!",
    }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(loginRes, { "login 200 or 401": (r) => r.status === 200 || r.status === 401 });

  // Profile fetch (if we got a token)
  if (loginRes.status === 200) {
    const token = JSON.parse(loginRes.body).token;
    const profile = http.get(`${BASE_URL}/api/v1/users/me`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    check(profile, { "profile 200": (r) => r.status === 200 });
  }

  sleep(1);
}
