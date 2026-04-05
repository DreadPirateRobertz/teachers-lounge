/**
 * @fileoverview Tests for the /api/flashcards Next.js route handlers.
 *
 * Verifies that GET and POST proxy correctly to the upstream gaming service,
 * forward the Authorization header derived from the tl_token cookie, and
 * pass upstream status codes through to the caller.
 *
 * @jest-environment node
 */

import { NextRequest } from 'next/server'
import { GET, POST } from './route'

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

const mockFetch = jest.fn()

beforeEach(() => {
  process.env.GAMING_SERVICE_URL = 'http://gaming-service:8083'
  global.fetch = mockFetch
})

afterEach(() => {
  mockFetch.mockReset()
  delete process.env.GAMING_SERVICE_URL
})

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Creates a NextRequest for the flashcards route with optional auth.
 *
 * @param method - HTTP method ('GET' | 'POST').
 * @param opts.token - Optional Bearer token value to inject via cookie.
 * @param opts.authHeader - Optional raw Authorization header value.
 * @param opts.body - Optional JSON body string (for POST requests).
 * @returns A NextRequest configured for the flashcards endpoint.
 */
function makeRequest(
  method: 'GET' | 'POST',
  opts: { token?: string; authHeader?: string; body?: string } = {},
): NextRequest {
  const url = 'http://localhost:3000/api/flashcards'
  const headers = new Headers()

  if (opts.authHeader) {
    headers.set('Authorization', opts.authHeader)
  }
  if (opts.token) {
    headers.set('Cookie', `tl_token=${opts.token}`)
  }
  if (opts.body) {
    headers.set('Content-Type', 'application/json')
  }

  return new NextRequest(url, {
    method,
    headers,
    body: opts.body ?? null,
  })
}

/**
 * Creates a minimal fetch response mock.
 *
 * @param payload - The JSON payload to return.
 * @param status - HTTP status code.
 * @returns A plain object shaped like a Response.
 */
function mockUpstreamResponse(payload: object, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => payload,
  }
}

// ---------------------------------------------------------------------------
// GET tests
// ---------------------------------------------------------------------------

describe('GET /api/flashcards', () => {
  it('proxies to /gaming/flashcards and passes Authorization from cookie', async () => {
    mockFetch.mockResolvedValueOnce(
      mockUpstreamResponse({ flashcards: [] }),
    )

    const req = makeRequest('GET', { token: 'my-secret-token' })
    const res = await GET(req)

    expect(mockFetch).toHaveBeenCalledTimes(1)

    const [url, init] = mockFetch.mock.calls[0]
    expect(url).toBe('http://gaming-service:8083/gaming/flashcards')
    expect((init as RequestInit).headers).toMatchObject({
      Authorization: 'Bearer my-secret-token',
    })

    expect(res.status).toBe(200)
    const body = await res.json()
    expect(body).toEqual({ flashcards: [] })
  })

  it('forwards upstream 401 back to the caller', async () => {
    mockFetch.mockResolvedValueOnce(
      mockUpstreamResponse({ detail: 'Unauthorized' }, 401),
    )

    const req = makeRequest('GET')
    const res = await GET(req)

    expect(res.status).toBe(401)
    const body = await res.json()
    expect(body).toEqual({ detail: 'Unauthorized' })
  })

  it('proxies without Authorization header when no token is present', async () => {
    mockFetch.mockResolvedValueOnce(mockUpstreamResponse({ flashcards: [] }))

    const req = makeRequest('GET') // no token, no auth header
    await GET(req)

    const [, init] = mockFetch.mock.calls[0]
    const headers = (init as RequestInit).headers as Record<string, string>
    expect(headers.Authorization).toBeUndefined()
  })

  it('prefers the Authorization header over the cookie when both are present', async () => {
    mockFetch.mockResolvedValueOnce(mockUpstreamResponse({ flashcards: [] }))

    const req = makeRequest('GET', {
      authHeader: 'Bearer header-token',
      token: 'cookie-token',
    })
    await GET(req)

    const [, init] = mockFetch.mock.calls[0]
    expect((init as RequestInit).headers).toMatchObject({
      Authorization: 'Bearer header-token',
    })
  })
})

// ---------------------------------------------------------------------------
// POST tests
// ---------------------------------------------------------------------------

describe('POST /api/flashcards', () => {
  it('proxies body to /gaming/flashcards/generate', async () => {
    const upstreamPayload = { generated: 5 }
    mockFetch.mockResolvedValueOnce(mockUpstreamResponse(upstreamPayload, 201))

    const reqBody = JSON.stringify({ topic: 'Photosynthesis', count: 5 })
    const req = makeRequest('POST', { token: 'tok', body: reqBody })
    const res = await POST(req)

    expect(mockFetch).toHaveBeenCalledTimes(1)

    const [url, init] = mockFetch.mock.calls[0]
    expect(url).toBe('http://gaming-service:8083/gaming/flashcards/generate')
    expect((init as RequestInit).method).toBe('POST')
    expect((init as RequestInit).body).toBe(reqBody)
    expect((init as RequestInit).headers).toMatchObject({
      'Content-Type': 'application/json',
      Authorization: 'Bearer tok',
    })

    expect(res.status).toBe(201)
    const body = await res.json()
    expect(body).toEqual(upstreamPayload)
  })

  it('proxies even when no auth token is provided (no 401 enforcement at this layer)', async () => {
    mockFetch.mockResolvedValueOnce(mockUpstreamResponse({ generated: 0 }, 200))

    const req = makeRequest('POST', { body: '{}' }) // no auth
    const res = await POST(req)

    expect(res.status).toBe(200)

    const [, init] = mockFetch.mock.calls[0]
    const headers = (init as RequestInit).headers as Record<string, string>
    expect(headers.Authorization).toBeUndefined()
  })

  it('forwards upstream error status codes through to the caller', async () => {
    mockFetch.mockResolvedValueOnce(
      mockUpstreamResponse({ detail: 'Bad request' }, 422),
    )

    const req = makeRequest('POST', { body: '{"bad": true}' })
    const res = await POST(req)

    expect(res.status).toBe(422)
    const body = await res.json()
    expect(body).toEqual({ detail: 'Bad request' })
  })
})
