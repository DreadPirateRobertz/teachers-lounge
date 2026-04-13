/**
 * @jest-environment node
 * @fileoverview Tests for PATCH /api/user/onboarding.
 *
 * Marks the authenticated user's onboarding wizard as complete.
 * Token validation and user-service proxying are tested here.
 */

import { NextRequest } from 'next/server'

const originalFetch = global.fetch
const originalEnv = process.env.USER_SERVICE_URL

// A minimal JWT whose payload decodes to `{ sub: "user-123" }`.
// Payload: base64url({ "sub": "user-123" })
const VALID_TOKEN =
  'eyJhbGciOiJIUzI1NiJ9.' +
  Buffer.from(JSON.stringify({ sub: 'user-123' })).toString('base64url') +
  '.sig'

function makeRequest(opts: { token?: string | null; serviceUrl?: string } = {}): NextRequest {
  const headers: Record<string, string> = {}
  const token = opts.token !== undefined ? opts.token : VALID_TOKEN
  if (token !== null) {
    headers['Cookie'] = `tl_token=${token}`
  }
  return new NextRequest('http://localhost/api/user/onboarding', {
    method: 'PATCH',
    headers,
  })
}

beforeEach(() => {
  process.env.USER_SERVICE_URL = 'http://user-service:8080'
  jest.resetModules()
})

afterEach(() => {
  global.fetch = originalFetch
  if (originalEnv !== undefined) {
    process.env.USER_SERVICE_URL = originalEnv
  } else {
    delete process.env.USER_SERVICE_URL
  }
})

// ── Authentication guard ──────────────────────────────────────────────────────

describe('PATCH /api/user/onboarding — auth guard', () => {
  it('returns 400 when tl_token cookie is absent', async () => {
    const { PATCH } = await import('./route')
    const resp = await PATCH(makeRequest({ token: null }))

    expect(resp.status).toBe(400)
    const data = await resp.json()
    expect(data.error).toBe('not authenticated')
  })

  it('returns 400 when token payload cannot be parsed', async () => {
    const { PATCH } = await import('./route')
    // A token with a non-base64url payload
    const badToken = 'header.!!!.sig'
    const resp = await PATCH(makeRequest({ token: badToken }))

    expect(resp.status).toBe(400)
    const data = await resp.json()
    expect(data.error).toBe('invalid token')
  })

  it('returns 400 when JWT sub claim is missing', async () => {
    const { PATCH } = await import('./route')
    // Valid base64url but no sub claim
    const noSubToken =
      'h.' + Buffer.from(JSON.stringify({ role: 'student' })).toString('base64url') + '.s'
    const resp = await PATCH(makeRequest({ token: noSubToken }))

    expect(resp.status).toBe(400)
    const data = await resp.json()
    expect(data.error).toBe('invalid token')
  })
})

// ── Upstream proxy ────────────────────────────────────────────────────────────

describe('PATCH /api/user/onboarding — user-service proxy', () => {
  it('proxies PATCH to user-service /users/{userId}/onboarding', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(new Response(null, { status: 204 }))
    })

    const { PATCH } = await import('./route')
    const resp = await PATCH(makeRequest())

    expect(resp.status).toBe(204)
    expect(capturedUrl).toContain('/users/user-123/onboarding')
  })

  it('forwards Bearer token in Authorization header', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(new Response(null, { status: 204 }))
    })

    const { PATCH } = await import('./route')
    await PATCH(makeRequest())

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${VALID_TOKEN}`)
  })

  it('returns 204 No Content on success', async () => {
    global.fetch = jest.fn().mockResolvedValue(new Response(null, { status: 204 }))

    const { PATCH } = await import('./route')
    const resp = await PATCH(makeRequest())

    expect(resp.status).toBe(204)
  })

  it('forwards JSON error body from upstream failure', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ detail: 'user not found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { PATCH } = await import('./route')
    const resp = await PATCH(makeRequest())

    expect(resp.status).toBe(404)
    const data = await resp.json()
    expect(data.detail).toBe('user not found')
  })

  it('forwards text error body from upstream failure', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response('internal server error', {
        status: 500,
        headers: { 'Content-Type': 'text/plain' },
      }),
    )

    const { PATCH } = await import('./route')
    const resp = await PATCH(makeRequest())

    expect(resp.status).toBe(500)
  })
})
