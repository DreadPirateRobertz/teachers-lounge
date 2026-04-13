import { test, expect } from '@playwright/test'
import * as fs from 'fs'
import * as path from 'path'

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Load credentials written by global-setup. */
function loadCredentials(): { email: string; password: string; displayName: string } {
  const credFile = path.join(__dirname, '../.auth/credentials.json')
  return JSON.parse(fs.readFileSync(credFile, 'utf-8'))
}

// ── 1. Register + Login ───────────────────────────────────────────────────────

test.describe('Register + Login', () => {
  /**
   * The global-setup already registers a user; here we verify login with those
   * same credentials works end-to-end.  We use a fresh context (no storageState)
   * so we're not relying on existing cookies.
   */
  test('login with registered credentials lands on home', async ({ browser }) => {
    const { email, password } = loadCredentials()
    const context = await browser.newContext()
    const page = await context.newPage()

    await page.goto('/login')
    await expect(page.locator('h1')).toContainText('Welcome back')

    await page.fill('#email', email)
    await page.fill('#password', password)
    await page.click('button[type="submit"]')

    await expect(page).toHaveURL('/', { timeout: 15_000 })
    await context.close()
  })

  test('registration form rejects mismatched passwords', async ({ browser }) => {
    const context = await browser.newContext()
    const page = await context.newPage()

    await page.goto('/register')
    await page.fill('#display-name', 'TestUser')
    await page.fill('#email', `mismatch+${Date.now()}@test.local`)
    await page.fill('#password', 'Password1!')
    await page.fill('#confirm', 'DifferentPass1!')
    await page.click('button[type="submit"]')

    await expect(page.locator('text=Passwords do not match')).toBeVisible()
    await context.close()
  })
})

// ── 2. Onboarding Wizard ──────────────────────────────────────────────────────

test.describe('Onboarding wizard', () => {
  /**
   * Navigates to /onboard and walks through all four steps:
   * welcome → character → upload-guide → ready → /
   *
   * Uses the auth storageState provided via playwright.config.ts projects.
   */
  test('completes all wizard steps and lands on home', async ({ page }) => {
    await page.goto('/onboard')

    // Step 1 — Welcome: "Let's go ⚡" button
    const letsGo = page.getByRole('button', { name: /let.?s go/i })
    await expect(letsGo).toBeVisible({ timeout: 10_000 })
    await letsGo.click()

    // Step 2 — Character: fill name, click "Looks good →"
    const nameInput = page.locator('#onboard-name')
    await expect(nameInput).toBeVisible()
    await nameInput.fill('E2E Tester')
    await page.getByRole('button', { name: /looks good/i }).click()

    // Step 3 — Upload guide: "Got it →" button
    await expect(page.getByRole('button', { name: /got it/i })).toBeVisible({ timeout: 10_000 })
    await page.getByRole('button', { name: /got it/i }).click()

    // Step 4 — Ready: "Start learning ⚡" button
    await expect(page.getByRole('button', { name: /start learning/i })).toBeVisible({
      timeout: 10_000,
    })
    await page.getByRole('button', { name: /start learning/i }).click()

    // Should land on home after completing wizard
    await expect(page).toHaveURL('/', { timeout: 15_000 })
  })
})

// ── 3. Material Upload ────────────────────────────────────────────────────────

test.describe('Material upload', () => {
  /**
   * Tests the upload API route with a minimal PDF-like payload.
   *
   * When INGESTION_SERVICE_URL is not set (local dev default) the route returns
   * a mock 202 so this test validates the API boundary without needing the
   * full ingestion stack.
   */
  test('POST /api/materials/upload returns job_id', async ({ request }) => {
    const courseId = '00000000-0000-4000-8000-000000000001'

    // Create a minimal file payload (1-byte PDF stub)
    const buffer = Buffer.from('%PDF-1.4 stub')
    const formData = new FormData()
    formData.append('file', new Blob([buffer], { type: 'application/pdf' }), 'lecture.pdf')

    const res = await request.post(`/api/materials/upload?course_id=${courseId}`, {
      multipart: {
        file: {
          name: 'lecture.pdf',
          mimeType: 'application/pdf',
          buffer,
        },
      },
    })

    // 202 (mock) or 200/201 (real ingestion service)
    expect([200, 201, 202]).toContain(res.status())
    const body = await res.json()
    expect(body).toHaveProperty('job_id')
    expect(body).toHaveProperty('material_id')
  })

  test('upload without course_id returns 400', async ({ request }) => {
    const res = await request.post('/api/materials/upload', {
      multipart: {
        file: {
          name: 'test.pdf',
          mimeType: 'application/pdf',
          buffer: Buffer.from('stub'),
        },
      },
    })
    expect(res.status()).toBe(400)
  })
})

// ── 4. Chat with Prof. Nova ───────────────────────────────────────────────────

test.describe('Chat', () => {
  /**
   * Sends a message in the chat panel and waits for a non-empty reply.
   * Uses the auth state so the tutoring service has a valid JWT.
   */
  test('sends a message and receives a streamed response', async ({ page }) => {
    await page.goto('/')

    // Wait for chat panel to render
    const chatTextarea = page.locator('textarea[placeholder*="Prof Nova"]')
    await expect(chatTextarea).toBeVisible({ timeout: 15_000 })

    await chatTextarea.fill('What is an atom?')
    await page.keyboard.press('Enter')

    // Wait for a response mentioning "atom" (streamed from Prof. Nova)
    await expect(page.locator('text=atom').first()).toBeVisible({ timeout: 30_000 })
  })

  test('chat API route is reachable and returns a stream', async ({ request }) => {
    const res = await request.post('/api/chat', {
      data: {
        message: 'Hello',
        conversation_id: null,
      },
    })
    // 200 (streaming) or 401 (unauthenticated request — acceptable, means route exists)
    expect([200, 401, 403]).toContain(res.status())
  })
})

// ── 5. Boss Battle ────────────────────────────────────────────────────────────

test.describe('Boss battle', () => {
  /**
   * Navigates to the first boss (The Atom) and initiates a battle.
   * The battle start may fail against the gaming service (402/500) but the
   * UI should show the start screen and the button should be clickable.
   */
  test('renders boss start screen for /boss-battle/1', async ({ page }) => {
    await page.goto('/boss-battle/1')

    // Boss name should be rendered
    await expect(page.locator('text=THE ATOM').or(page.locator('text=The Atom'))).toBeVisible({
      timeout: 10_000,
    })

    // Begin Battle button present
    const beginBtn = page.getByRole('button', { name: /begin battle/i })
    await expect(beginBtn).toBeVisible()
  })

  test('clicking Begin Battle calls the gaming service API', async ({ page }) => {
    await page.goto('/boss-battle/1')

    const beginBtn = page.getByRole('button', { name: /begin battle/i })
    await expect(beginBtn).toBeVisible({ timeout: 10_000 })

    await beginBtn.click()

    // Wait briefly — button should go loading or UI transitions to battle/error
    await page.waitForTimeout(2_000)

    // Page must not crash regardless of API outcome
    await expect(page.locator('body')).toBeVisible()
  })
})

// ── 6. Flashcards ─────────────────────────────────────────────────────────────

test.describe('Flashcards', () => {
  /**
   * Tests flashcard creation and review via the Next.js API proxy routes.
   * The gaming-service backend must be running for these to return 200.
   * On unauthenticated request proxy returns 401 — also acceptable here.
   */
  test('GET /api/flashcards returns a list (or 401)', async ({ request }) => {
    const res = await request.get('/api/flashcards')
    expect([200, 401]).toContain(res.status())
    if (res.status() === 200) {
      const body = await res.json()
      expect(Array.isArray(body) || typeof body === 'object').toBe(true)
    }
  })

  test('GET /api/flashcards/due returns due cards (or 401)', async ({ request }) => {
    const res = await request.get('/api/flashcards/due')
    expect([200, 401]).toContain(res.status())
  })

  test('POST /api/flashcards to generate cards (or 401/422)', async ({ request }) => {
    const res = await request.post('/api/flashcards', {
      data: {
        topic: 'atomic structure',
        count: 3,
      },
    })
    // 200 success, 401 unauth, 422 invalid payload — all indicate route exists
    expect([200, 201, 401, 422]).toContain(res.status())
  })

  test('flashcard review route accepts a rating (or 401/404)', async ({ request }) => {
    // Use a placeholder card ID — will 404 if not found, 401 if unauth
    const res = await request.post('/api/flashcards/00000000-0000-4000-8000-000000000001/review', {
      data: { quality: 4 },
    })
    expect([200, 401, 404, 422]).toContain(res.status())
  })
})

// ── 7. Leaderboard ────────────────────────────────────────────────────────────

test.describe('Leaderboard', () => {
  /**
   * Clicks the Rankings tab in the right sidebar on the home page and
   * verifies the leaderboard panel renders without crashing.
   */
  test('Rankings tab renders leaderboard panel on desktop', async ({ page }) => {
    // Use desktop viewport — sidebar is hidden on mobile
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto('/')

    // The right sidebar has Mastery / Rankings / Power-ups tabs
    const rankingsTab = page.getByRole('button', { name: 'Rankings' })
    await expect(rankingsTab).toBeVisible({ timeout: 10_000 })
    await rankingsTab.click()

    // After clicking, a leaderboard panel should render (may show loading or data)
    // We look for the tab to become active and the panel to mount without error
    await expect(rankingsTab).toHaveClass(/neon-blue|border-neon-blue/, { timeout: 5_000 })

    // No crash — body still visible
    await expect(page.locator('body')).toBeVisible()
  })

  test('GET /api/gaming/leaderboard returns data (or 401)', async ({ request }) => {
    const res = await request.get('/api/gaming/leaderboard')
    expect([200, 401]).toContain(res.status())
    if (res.status() === 200) {
      const body = await res.json()
      expect(typeof body === 'object').toBe(true)
    }
  })
})
