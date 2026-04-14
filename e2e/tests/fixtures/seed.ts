import * as fs from 'fs'
import * as path from 'path'
import * as crypto from 'crypto'

const BASE_URL = process.env.BASE_URL || 'http://localhost:3000'

/**
 * Credentials for a seeded test user.
 */
export interface SeedUser {
  email: string
  password: string
  displayName: string
  token: string
}

/**
 * Register a unique test user via the app's registration endpoint and return
 * a JWT token.  Each call creates a distinct user so tests are hermetic.
 */
export async function registerUser(suffix?: string): Promise<SeedUser> {
  const uid = suffix ?? crypto.randomBytes(6).toString('hex')
  const email = `seed+${uid}@test.local`
  const password = 'Seed_pw1!'
  const displayName = `SeedUser_${uid}`

  const res = await fetch(`${BASE_URL}/api/user/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, display_name: displayName }),
  })
  if (!res.ok && res.status !== 409) {
    throw new Error(`registerUser failed: ${res.status} ${await res.text()}`)
  }

  // Login to get a token (handles both new and pre-existing users)
  const loginRes = await fetch(`${BASE_URL}/api/user/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!loginRes.ok) {
    throw new Error(`login failed: ${loginRes.status} ${await loginRes.text()}`)
  }

  const data = await loginRes.json()
  const token: string = data.token ?? data.access_token ?? data.jwt ?? ''
  return { email, password, displayName, token }
}

/**
 * Seed a study event for a user so the gaming service increments their streak.
 *
 * @param token - Bearer token for the authenticated user.
 */
export async function seedStudyEvent(token: string): Promise<void> {
  const res = await fetch(`${BASE_URL}/api/gaming/progression`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ event_type: 'study_session', xp: 10 }),
  })
  // 200/201 = success, 401/422/500 = service not available — silently skip
  if (res.status >= 500) {
    console.warn(`seedStudyEvent: upstream returned ${res.status}, continuing`)
  }
}

/**
 * Seed five leaderboard entries so the leaderboard panel renders with data.
 *
 * Uses the global credentials file written by global-setup if available,
 * otherwise creates fresh users.
 */
export async function seedLeaderboard(): Promise<void> {
  for (let i = 0; i < 5; i++) {
    try {
      const user = await registerUser(`lb${i}_${Date.now()}`)
      await seedStudyEvent(user.token)
    } catch {
      // Best-effort — leaderboard may already have entries
    }
  }
}

/**
 * Load the auth credentials written by global-setup.
 */
export function loadGlobalCredentials(): { email: string; password: string; displayName: string } {
  const credFile = path.join(__dirname, '../../.auth/credentials.json')
  return JSON.parse(fs.readFileSync(credFile, 'utf-8'))
}

/**
 * Return the bearer token from a Playwright storageState file.
 *
 * Playwright's `storageState` stores cookies.  The token cookie is `tl_token`.
 */
export function tokenFromStorageState(stateFile?: string): string {
  const file = stateFile ?? path.join(__dirname, '../../.auth/state.json')
  try {
    const state = JSON.parse(fs.readFileSync(file, 'utf-8'))
    const cookie = (state.cookies as Array<{ name: string; value: string }>).find(
      (c) => c.name === 'tl_token',
    )
    return cookie?.value ?? ''
  } catch {
    return ''
  }
}
