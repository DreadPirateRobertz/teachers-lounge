/**
 * @jest-environment node
 * @fileoverview Tests for the /api/flashcards Next.js route handlers.
 *
 * GET proxies to gaming-service /gaming/flashcards.
 * POST proxies to gaming-service /gaming/flashcards/generate.
 * Both endpoints forward the auth token from the Authorization header or
 * tl_token cookie.
 */

import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'flashcard-test-token'
const originalFetch = global.fetch

function makeGetRequest(opts: { token?: string | null } = {}): NextRequest {
  const headers: Record<string, string> = {}
  const token = opts.token !== undefined ? opts.token : MOCK_TOKEN
  if (token !== null) {
    headers['Cookie'] = `tl_token=${token}`
  }
  return new NextRequest('http://localhost/api/flashcards', { method: 'GET', headers })
}

function makePostRequest(opts: { body?: unknown; token?: string | null } = {}): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = opts.token !== undefined ? opts.token : MOCK_TOKEN
  if (token !== null) {
    headers['Cookie'] = `tl_token=${token}`
  }
  return new NextRequest('http://localhost/api/flashcards', {
    method: 'POST',
    headers,
    body: JSON.stringify(opts.body ?? { material_id: 'mat-1', count: 5 }),
  })
}

beforeEach(() => {
  process.env.GAMING_SERVICE_URL = 'http://gaming-service:8083'
  jest.resetModules()
})

afterEach(() => {
  global.fetch = originalFetch
  delete process.env.GAMING_SERVICE_URL
})

// ── GET /api/flashcards ───────────────────────────────────────────────────────

describe('GET /api/flashcards', () => {
  it('proxies GET to gaming-service /gaming/flashcards', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ cards: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    const resp = await GET(makeGetRequest())
    const data = await resp.json()

    expect(resp.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/flashcards')
    expect(data.cards).toBeDefined()
  })

  it('forwards upstream status code on error', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ detail: 'not found' }), { status: 404 }),
    )

    const { GET } = await import('./route')
    const resp = await GET(makeGetRequest())

    expect(resp.status).toBe(404)
  })

  it('forwards Authorization header from tl_token cookie', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ cards: [] }), { status: 200 }),
      )
    })

    const { GET } = await import('./route')
    await GET(makeGetRequest({ token: 'my-jwt' }))

    expect(capturedHeaders['Authorization']).toBe('Bearer my-jwt')
  })

  it('forwards Authorization header from Authorization request header', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ cards: [] }), { status: 200 }),
      )
    })

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards', {
      headers: { Authorization: 'Bearer header-token' },
    })
    await GET(req)

    expect(capturedHeaders['Authorization']).toBe('Bearer header-token')
  })

  it('sends no Authorization header when unauthenticated', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ cards: [] }), { status: 200 }),
      )
    })

    const { GET } = await import('./route')
    await GET(makeGetRequest({ token: null }))

    expect(capturedHeaders['Authorization']).toBeUndefined()
  })
})

// ── POST /api/flashcards ──────────────────────────────────────────────────────

describe('POST /api/flashcards', () => {
  it('proxies POST to gaming-service /gaming/flashcards/generate', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ cards: [{ id: 'c1', front: 'Q', back: 'A' }] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    const resp = await POST(makePostRequest({ body: { material_id: 'mat-1', count: 3 } }))
    const data = await resp.json()

    expect(resp.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/flashcards/generate')
    expect(Array.isArray(data.cards)).toBe(true)
  })

  it('forwards upstream status code on POST error', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'bad request' }), { status: 400 }),
    )

    const { POST } = await import('./route')
    const resp = await POST(makePostRequest())

    expect(resp.status).toBe(400)
  })

  it('forwards Authorization header on POST from tl_token cookie', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ cards: [] }), { status: 200 }),
      )
    })

    const { POST } = await import('./route')
    await POST(makePostRequest({ token: 'post-cookie-token' }))

    expect(capturedHeaders['Authorization']).toBe('Bearer post-cookie-token')
  })

  it('sends no Authorization header on POST when unauthenticated', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ cards: [] }), { status: 200 }),
      )
    })

    const { POST } = await import('./route')
    await POST(makePostRequest({ token: null }))

    expect(capturedHeaders['Authorization']).toBeUndefined()
  })
})
