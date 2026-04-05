/**
 * k6 load test — critical path: boss battle flow.
 *
 * Path: POST /boss/start → POST /boss/attack (5 rounds) → POST /boss/forfeit
 * Target: 500 concurrent battles, p99 round-trip <300ms.
 *
 * Usage:
 *   k6 run tests/load/boss-battle.js \
 *     --env BASE_URL=https://api-staging.teacherslounge.app \
 *     --env AUTH_TOKEN=<staging-service-token>
 */
import http from "k6/http";
import { check, group, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8083";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "test-token";

const battleStartDuration = new Trend("battle_start_ms");
const attackRoundDuration = new Trend("attack_round_ms");
const forfeitDuration = new Trend("battle_forfeit_ms");
const battleErrorRate = new Rate("battle_errors");

// Boss IDs available in staging seed data
const BOSS_IDS = ["boss_algebra", "boss_biology", "boss_history", "boss_physics"];

export const options = {
  stages: [
    { duration: "2m", target: 100 },  // ramp to 100 battles
    { duration: "3m", target: 500 },  // ramp to 500 battles (peak)
    { duration: "5m", target: 500 },  // sustain peak
    { duration: "2m", target: 0 },    // cool down
  ],
  thresholds: {
    // Primary SLO: p99 round-trip <300ms for each battle action
    "battle_start_ms": ["p(99)<300"],
    "attack_round_ms": ["p(99)<300"],
    "battle_forfeit_ms": ["p(99)<300"],
    battle_errors: ["rate<0.01"],
    http_req_failed: ["rate<0.01"],
  },
};

export default function () {
  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${AUTH_TOKEN}`,
  };

  const bossId = BOSS_IDS[__VU % BOSS_IDS.length];
  let sessionId = null;

  group("battle_start", () => {
    const res = http.post(
      `${BASE_URL}/api/v1/boss/start`,
      JSON.stringify({ boss_id: bossId }),
      { headers }
    );
    battleStartDuration.add(res.timings.duration);

    const ok = check(res, {
      "battle start 200": (r) => r.status === 200,
      "session id returned": (r) => {
        try {
          return !!JSON.parse(r.body).session_id;
        } catch {
          return false;
        }
      },
    });
    battleErrorRate.add(!ok);

    if (res.status === 200) {
      sessionId = JSON.parse(res.body).session_id;
    }
  });

  if (!sessionId) {
    sleep(1);
    return;
  }

  // Simulate 5 attack rounds
  let battleOver = false;
  for (let round = 0; round < 5 && !battleOver; round++) {
    group("attack_round", () => {
      // Randomly choose correct/incorrect to simulate real gameplay
      const answerCorrect = Math.random() > 0.4;

      const res = http.post(
        `${BASE_URL}/api/v1/boss/attack`,
        JSON.stringify({
          session_id: sessionId,
          answer_correct: answerCorrect,
          // question_id would come from the start response in real flow
          question_id: `q${round + 1}`,
        }),
        { headers }
      );
      attackRoundDuration.add(res.timings.duration);

      const ok = check(res, {
        "attack 200": (r) => r.status === 200,
      });
      battleErrorRate.add(!ok);

      if (res.status === 200) {
        try {
          const body = JSON.parse(res.body);
          // Battle may end naturally (victory/defeat) before 5 rounds
          if (body.battle_over) {
            battleOver = true;
          }
        } catch {
          // continue
        }
      }
    });

    // Short think-time between attack rounds (student reading question)
    sleep(Math.random() * 1.5 + 0.5);
  }

  // Forfeit if battle is still active after 5 rounds (cleanup)
  if (!battleOver) {
    group("battle_forfeit", () => {
      const res = http.post(
        `${BASE_URL}/api/v1/boss/forfeit`,
        JSON.stringify({ session_id: sessionId }),
        { headers }
      );
      forfeitDuration.add(res.timings.duration);
      const ok = check(res, { "forfeit 200": (r) => r.status === 200 });
      battleErrorRate.add(!ok);
    });
  }

  sleep(Math.random() * 2 + 1);
}
