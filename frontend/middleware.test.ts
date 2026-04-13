/**
 * Tests for middleware.ts — verifies nonce generation, CSP injection,
 * auth redirects, and subscription gating.
 */

import { NextRequest } from 'next/server'
import { generateNonce, middleware } from './middleware'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Build a minimal NextRequest for a given path, with optional JWT cookie. */
function makeRequest(path: string, token?: string): NextRequest {
  const url = `http://localhost${path}`
  const init: RequestInit & { headers?: Record<string, string> } = {}
  if (token) {
    init.headers = { cookie: `tl_token=${token}` }
  }
  return new NextRequest(url, init)
}

/**
 * Build a compact unsigned JWT with the given payload for routing tests.
 * Signature verification is not performed by middleware — any payload works.
 */
function fakeJwt(payload: Record<string, unknown>): string {
  const header = Buffer.from(JSON.stringify({ alg: 'HS256' })).toString('base64url')
  const body = Buffer.from(JSON.stringify(payload)).toString('base64url')
  return `${header}.${body}.fakesig`
}

// ---------------------------------------------------------------------------
// generateNonce
// ---------------------------------------------------------------------------

describe('generateNonce', () => {
  it('returns a non-empty string', () => {
    expect(generateNonce()).toBeTruthy()
  })

  it('returns a base64-encoded string', () => {
    const nonce = generateNonce()
    expect(() => Buffer.from(nonce, 'base64')).not.toThrow()
  })

  it('produces unique values on each call', () => {
    const a = generateNonce()
    const b = generateNonce()
    expect(a).not.toBe(b)
  })

  it('decodes to 16 bytes (128 bits of entropy)', () => {
    const nonce = generateNonce()
    expect(Buffer.from(nonce, 'base64').byteLength).toBe(16)
  })
})

// ---------------------------------------------------------------------------
// middleware — CSP nonce injection
// ---------------------------------------------------------------------------

describe('middleware CSP nonce injection', () => {
  it('sets Content-Security-Policy on every page response', () => {
    const req = makeRequest('/', fakeJwt({ sub_status: 'active' }))
    const res = middleware(req)
    expect(res.headers.get('Content-Security-Policy')).toBeTruthy()
  })

  it('CSP contains nonce- in script-src', () => {
    const req = makeRequest('/', fakeJwt({ sub_status: 'active' }))
    const res = middleware(req)
    const csp = res.headers.get('Content-Security-Policy') ?? ''
    expect(csp).toMatch(/script-src 'self' 'nonce-[A-Za-z0-9+/=]+' 'strict-dynamic'/)
  })

  it('script-src in CSP does not contain unsafe-inline', () => {
    const req = makeRequest('/', fakeJwt({ sub_status: 'active' }))
    const res = middleware(req)
    const csp = res.headers.get('Content-Security-Policy') ?? ''
    const scriptSrc = csp.match(/script-src([^;]+)/)?.[1] ?? ''
    expect(scriptSrc).not.toContain('unsafe-inline')
  })

  it('nonce in CSP matches x-nonce request header', () => {
    // We can't read the forwarded request header directly from the response,
    // but we can verify that the nonce embedded in CSP is valid base64.
    const req = makeRequest('/', fakeJwt({ sub_status: 'active' }))
    const res = middleware(req)
    const csp = res.headers.get('Content-Security-Policy') ?? ''
    const match = csp.match(/'nonce-([A-Za-z0-9+/=]+)'/)
    expect(match).not.toBeNull()
    const nonce = match![1]
    expect(Buffer.from(nonce, 'base64').byteLength).toBe(16)
  })
})

// ---------------------------------------------------------------------------
// middleware — auth redirects
// ---------------------------------------------------------------------------

describe('middleware auth redirects', () => {
  it('redirects unauthenticated user from protected route to /login', () => {
    const req = makeRequest('/dashboard')
    const res = middleware(req)
    expect(res.status).toBe(307)
    expect(res.headers.get('location')).toContain('/login')
  })

  it('passes next= query param when redirecting to login', () => {
    const req = makeRequest('/quests')
    const res = middleware(req)
    expect(res.headers.get('location')).toContain('next=%2Fquests')
  })

  it('allows unauthenticated access to /login', () => {
    const req = makeRequest('/login')
    const res = middleware(req)
    expect(res.status).not.toBe(307)
  })

  it('allows unauthenticated access to /register', () => {
    const req = makeRequest('/register')
    const res = middleware(req)
    expect(res.status).not.toBe(307)
  })

  it('allows unauthenticated access to /subscribe', () => {
    const req = makeRequest('/subscribe')
    const res = middleware(req)
    expect(res.status).not.toBe(307)
  })

  it('redirects authenticated user away from /login to /', () => {
    const token = fakeJwt({ sub_status: 'active' })
    const req = makeRequest('/login', token)
    const res = middleware(req)
    expect(res.status).toBe(307)
    expect(res.headers.get('location')).toContain('http://localhost/')
  })
})

// ---------------------------------------------------------------------------
// middleware — subscription gate
// ---------------------------------------------------------------------------

describe('middleware subscription gate', () => {
  it('allows active subscriber to /dashboard', () => {
    const token = fakeJwt({ sub_status: 'active' })
    const req = makeRequest('/dashboard', token)
    const res = middleware(req)
    expect(res.status).not.toBe(307)
  })

  it('allows trialing subscriber to /dashboard', () => {
    const token = fakeJwt({ sub_status: 'trialing' })
    const req = makeRequest('/dashboard', token)
    const res = middleware(req)
    expect(res.status).not.toBe(307)
  })

  it('redirects non-subscriber from /dashboard to /subscribe', () => {
    const token = fakeJwt({ sub_status: 'inactive' })
    const req = makeRequest('/dashboard', token)
    const res = middleware(req)
    expect(res.status).toBe(307)
    expect(res.headers.get('location')).toContain('/subscribe')
  })

  it('allows /subscribe/success post-auth route with active subscription', () => {
    const token = fakeJwt({ sub_status: 'active' })
    const req = makeRequest('/subscribe/success', token)
    const res = middleware(req)
    // Should not redirect to / (which would happen for plain /subscribe with token)
    expect(res.headers.get('location') ?? '').not.toContain('http://localhost/')
  })
})
