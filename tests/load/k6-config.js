// Shared k6 configuration for TeachersLounge load tests.
// Usage: k6 run tests/load/<service>.js --env BASE_URL=https://api.teacherslounge.app

export const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

// Standard load stages: ramp up → sustained → spike → cool down
export const standardStages = [
  { duration: "2m", target: 50 },   // ramp to 50 VUs
  { duration: "5m", target: 50 },   // sustain 50 VUs
  { duration: "1m", target: 150 },  // spike to 150 VUs
  { duration: "3m", target: 150 },  // sustain spike
  { duration: "2m", target: 50 },   // back to baseline
  { duration: "1m", target: 0 },    // ramp down
];

// Soak test stages: longer sustained load for HPA validation
export const soakStages = [
  { duration: "2m", target: 50 },
  { duration: "15m", target: 50 },
  { duration: "2m", target: 100 },
  { duration: "15m", target: 100 },
  { duration: "2m", target: 0 },
];

// Thresholds aligned with HPA targets — if these fail, HPA tuning is needed
export const standardThresholds = {
  http_req_duration: ["p(95)<500", "p(99)<1500"],
  http_req_failed: ["rate<0.01"],
  http_reqs: ["rate>10"],
};

export const streamingThresholds = {
  http_req_duration: ["p(95)<5000", "p(99)<15000"],
  http_req_failed: ["rate<0.02"],
};
