import { test as setup, expect } from '@playwright/test'
import * as fs from 'fs'
import * as path from 'path'

const AUTH_FILE = path.join(__dirname, '../.auth/state.json')

/**
 * Global setup — runs once before all smoke tests.
 *
 * Registers a fresh test user then saves the browser storage state
 * (cookies including tl_token) so subsequent tests start authenticated.
 * A timestamp-based email ensures uniqueness across test runs.
 */
setup('register test user', async ({ page }) => {
  const ts = Date.now()
  const email = `e2e+${ts}@test.local`
  const password = 'Smoke_test_pw1!'
  const displayName = `SmokeUser${ts}`

  // Store credentials in env so individual tests can login if needed
  process.env.E2E_EMAIL = email
  process.env.E2E_PASSWORD = password
  process.env.E2E_DISPLAY_NAME = displayName

  // Write credentials to a temp file for cross-process access
  const credDir = path.join(__dirname, '../.auth')
  fs.mkdirSync(credDir, { recursive: true })
  fs.writeFileSync(
    path.join(credDir, 'credentials.json'),
    JSON.stringify({ email, password, displayName }),
  )

  // Register via the API directly — dodges dev-mode CSP that blocks the
  // client-side submit handler in `next dev`. The response sets `tl_token`
  // as an httpOnly cookie which we then inject into the browser context.
  const apiResp = await page.request.post('/api/user/auth/register', {
    data: { email, password, display_name: displayName },
  })
  expect(apiResp.ok(), await apiResp.text()).toBeTruthy()

  // Visit the app so the cookie (set on the request context) is scoped to
  // this origin in the storage state snapshot.
  await page.goto('/')
  await expect(page).toHaveURL('/', { timeout: 15_000 })

  // Persist auth state for downstream tests
  await page.context().storageState({ path: AUTH_FILE })
})
