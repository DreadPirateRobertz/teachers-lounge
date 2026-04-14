# Testing-Guide Screenshots

Captured 2026-04-13 from pop-os Docker stack + Next.js dev server against
a freshly-registered demo user (`demo@example.com`). Viewport 1440×900,
full-page capture via Playwright / chromium-headless-shell.

## Files

| # | File | Route | Status |
|---|------|-------|--------|
| 1 | `01-docker-compose-ps.txt` | `docker compose ps` | all 9 reachable services healthy (qdrant healthcheck broken but serving; ingestion/search skipped, depend on qdrant healthcheck) |
| 2 | `02-boss-battle-entry.png` | `/boss-battle/the_atom` | ✅ title card + "Begin Battle" CTA |
| 3 | `03-boss-battle-active.png` | `/boss-battle/the_atom` after click | ⚠️ identical to entry — dev-mode CSP (`unsafe-eval`) blocks client hydration; needs production build |
| 4 | `04-chat-interface.png` | `/` | ✅ full shell: chat panel, mastery panel, daily quests, achievements, level-up modal |
| 5 | `05-leaderboard.png` | `/progression` | ⚠️ cold-start empty state — new user has no progression entries |
| 6 | `06-adaptive-dashboard.png` | `/adaptive` | ⚠️ cold-start empty state — new user has no mastery heatmap data |
| 7 | `07-shop.png` | `/shop` | ⚠️ cold-start empty state — shop items endpoint returned no data for new user |

## Known gaps

- **Loot reveal** (`LootReveal` from PR #219) not captured — only renders mid-battle after victory; requires seeded session + answering rounds.
- **Active battle with HP bars / Three.js scene** not captured — dev-mode CSP blocks hydration (see #3 above).
- **Leaderboard / mastery heatmap / shop items** need seed data: at minimum a few gaming rounds, mastery events, and shop catalog entries for the demo user.

## How these were produced

```bash
# Stack
cd teachers-lounge
docker compose up -d postgres redis user-service gaming-service \
  tutoring-service notification-service analytics-service ai-gateway

# Frontend in dev mode (against localhost-mapped services)
cd frontend
USER_SERVICE_URL=http://localhost:8080 \
TUTORING_SERVICE_URL=http://localhost:8000 \
GAMING_SERVICE_URL=http://localhost:8083 \
NOTIFICATION_SERVICE_URL=http://localhost:9000 \
ANALYTICS_SERVICE_URL=http://localhost:8085 \
AI_GATEWAY_URL=http://localhost:4000 \
PORT=3000 npm run dev

# Capture
npx playwright install chromium
node capture-screenshots.mjs
```

## Next pass

To fill the gaps, follow-up work needs either:
1. A production build (`npm run build && npm start`) to bypass dev-mode CSP, **or**
2. A seed script that registers a user, plays a boss battle through victory, populates mastery events and leaderboard ranks, and populates the shop catalog — then a Playwright flow that drives the UI through each screen.

Capture script is committed alongside (`frontend/capture-screenshots.mjs`) so the next run can extend it.
