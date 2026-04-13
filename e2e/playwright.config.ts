import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright configuration for TeachersLounge E2E smoke tests.
 *
 * Targets a running docker-compose stack at localhost:3000.
 * Run `docker compose --env-file .env.local up --build` first.
 */
export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: [['list'], ['html', { open: 'never' }]],
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
    // Smoke tests — depend on auth state from setup
    {
      name: 'smoke',
      dependencies: ['setup'],
      use: {
        ...devices['Desktop Chrome'],
        storageState: '.auth/state.json',
      },
    },
  ],
})
