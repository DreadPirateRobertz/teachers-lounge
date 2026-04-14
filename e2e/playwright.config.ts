import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright configuration for TeachersLounge E2E test suite.
 *
 * Targets a running docker-compose stack at localhost:3000.
 * Run `docker compose --env-file .env.local up --build` first.
 *
 * Screenshots are captured inline at key assertion points.
 * Run `node scripts/gen-testing-doc.mjs` after tests to regenerate
 * docs/testing-master.html with fresh screenshots and pass/fail status.
 */
export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: [
    ['list'],
    ['html', { open: 'never' }],
    // JSON reporter powers gen-testing-doc.mjs
    ['json', { outputFile: 'test-results/results.json' }],
  ],
  timeout: 60_000,

  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:3000',
    // Treat localhost as secure so httpOnly+secure cookies are sent over HTTP
    ignoreHTTPSErrors: true,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    trace: 'retain-on-failure',
  },

  projects: [
    // Global setup — registers a test user and saves auth state
    {
      name: 'setup',
      testMatch: '**/global-setup.ts',
    },
    // Full E2E suite — auth, tutoring, gaming, notifications
    {
      name: 'e2e',
      dependencies: ['setup'],
      testMatch: ['**/auth.spec.ts', '**/tutoring.spec.ts', '**/gaming.spec.ts', '**/notifications.spec.ts'],
      use: {
        ...devices['Desktop Chrome'],
        storageState: '.auth/state.json',
      },
    },
    // Legacy smoke tests — kept for backwards compat
    {
      name: 'smoke',
      dependencies: ['setup'],
      testMatch: '**/smoke.spec.ts',
      use: {
        ...devices['Desktop Chrome'],
        storageState: '.auth/state.json',
      },
    },
  ],
})
