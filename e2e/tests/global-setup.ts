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

  await page.goto('/register')
  await expect(page).toHaveURL('/register')

  await page.fill('#display-name', displayName)
  await page.fill('#email', email)
  await page.fill('#password', password)
  await page.fill('#confirm', password)
  await page.click('button[type="submit"]')

  // After register the token cookie is set; middleware redirects away from
  // /subscribe (a public path) to / for authenticated users.
  await expect(page).toHaveURL('/', { timeout: 15_000 })

  // Persist auth state for downstream tests
  await page.context().storageState({ path: AUTH_FILE })
})
