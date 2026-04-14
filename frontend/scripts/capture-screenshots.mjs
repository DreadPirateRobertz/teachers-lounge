/**
 * Capture testing-guide screenshots from the running teachers-lounge stack.
 * Requires the demo user to already exist (registered via /api/user/auth/register).
 */
import { chromium } from 'playwright'
import { mkdir } from 'node:fs/promises'
import { writeFileSync } from 'node:fs'

const BASE = 'http://localhost:3000'
const OUT = '/tmp/tl-dye-wt/docs/screenshots'

const CREDS = { email: 'demo@example.com', password: 'DemoPass123!', display_name: 'Demo User' }

async function ensureLoggedIn(context) {
  // Try login first; register if it fails.
  const r = await fetch(`${BASE}/api/user/auth/login`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ email: CREDS.email, password: CREDS.password }),
  })
  let setCookie = r.headers.get('set-cookie')
  if (r.status !== 200) {
    const rr = await fetch(`${BASE}/api/user/auth/register`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(CREDS),
    })
    setCookie = rr.headers.get('set-cookie')
  }
  if (!setCookie) throw new Error('no tl_token cookie from auth')
  const match = setCookie.match(/tl_token=([^;]+)/)
  if (!match) throw new Error('tl_token missing in set-cookie')
  await context.addCookies([
    {
      name: 'tl_token',
      value: match[1],
      domain: 'localhost',
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    },
  ])
}

async function shot(page, path, file) {
  console.log(`→ ${path}`)
  await page.goto(`${BASE}${path}`, { waitUntil: 'networkidle', timeout: 20_000 }).catch(() => {})
  await page.waitForTimeout(1500)
  await page.screenshot({ path: `${OUT}/${file}`, fullPage: true })
  console.log(`  saved ${file}`)
}

;(async () => {
  await mkdir(OUT, { recursive: true })
  const browser = await chromium.launch()
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  await ensureLoggedIn(context)
  const page = await context.newPage()
  page.on('pageerror', (e) => console.log('pageerror:', e.message))

  await shot(page, '/', '04-chat-interface.png')
  await shot(page, '/shop', '07-shop.png')
  await shot(page, '/adaptive', '06-adaptive-dashboard.png')
  await shot(page, '/progression', '05-leaderboard.png')
  await shot(page, '/boss-battle/the_atom', '02-boss-battle-entry.png')
  await shot(page, '/boss-battle/the_atom?phase=active', '03-boss-battle-active.png')
  await shot(page, '/boss-battle/the_atom?phase=victory', '04b-loot-reveal.png')

  await browser.close()
  console.log('done')
})().catch((e) => {
  console.error('FAILED:', e)
  process.exit(1)
})
