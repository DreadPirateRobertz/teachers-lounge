import http from "k6/http";
import { check, sleep } from "k6";
import { standardStages, standardThresholds } from "./k6-config.js";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8083";
const TOKEN = __ENV.AUTH_TOKEN || "test-token";

export const options = {
  stages: standardStages,
  thresholds: standardThresholds,
};

export default function () {
  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${TOKEN}`,
  };

  // Health check
  const health = http.get(`${BASE_URL}/health`);
  check(health, { "health 200": (r) => r.status === 200 });

  // Leaderboard fetch (read-heavy, cache-friendly)
  const leaderboard = http.get(`${BASE_URL}/api/v1/leaderboard?limit=10`, {
    headers,
  });
  check(leaderboard, {
    "leaderboard 200 or 401": (r) => r.status === 200 || r.status === 401,
  });

  // XP event (write path)
  const xp = http.post(
    `${BASE_URL}/api/v1/xp`,
    JSON.stringify({
      action: "lesson_complete",
      metadata: { lesson_id: `load-${__ITER}` },
    }),
    { headers }
  );
  check(xp, {
    "xp 200 or 401": (r) => r.status === 200 || r.status === 401,
  });

  // Streak check
  const streak = http.get(`${BASE_URL}/api/v1/streaks/me`, { headers });
  check(streak, {
    "streak 200 or 401": (r) => r.status === 200 || r.status === 401,
  });

  sleep(1);
}
