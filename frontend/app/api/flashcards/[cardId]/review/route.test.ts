/**
 * Tests for POST /api/flashcards/{cardId}/review
 * Mocks upstream fetch to gaming-service, verifies cardId routing, auth forwarding,
 * and SM-2 quality payload forwarding.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'test-token-review'
const GAMING_URL = 'http://gaming-service:8083'

type Params = { params: Promise<{ cardId: string }> }

function makeParams(cardId: string): Params {
  return { params: Promise.resolve({ cardId }) }
}

function makeRequest(opts: { cardId?: string; body?: unknown; token?: string | null } = {}): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.token !== null) {
    headers['Authorization'] = `Bearer ${opts.token ?? MOCK_TOKEN}`
  }
  return new NextRequest(
    `http://localhost/api/flashcards/${opts.cardId ?? 'card-1'}/review`,
    {
      method: 'POST',
      headers,
      body: JSON.stringify(opts.body ?? { quality: 4 }),
    },
  )
}

describe('POST /api/flashcards/{cardId}/review', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('routes to correct cardId URL on gaming-service', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ next_interval: 6 }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    const req = makeRequest({ cardId: 'card-42' })
    await POST(req, makeParams('card-42'))

    expect(capturedUrl).toBe(`${GAMING_URL}/gaming/flashcards/card-42/review`)
  })

  it('forwards quality payload to gaming-service', async () => {
    let capturedBody: unknown
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedBody = JSON.parse(init.body as string)
      return Promise.resolve(
        new Response(JSON.stringify({ next_interval: 1 }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    const req = makeRequest({ cardId: 'card-1', body: { quality: 2 } })
    await POST(req, makeParams('card-1'))

    expect((capturedBody as { quality: number }).quality).toBe(2)
  })

  it('returns updated card state from gaming-service', async () => {
    const updated = { id: 'card-1', ease_factor: 2.3, interval: 6, next_due: '2026-04-11' }
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify(updated), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const req = makeRequest({ cardId: 'card-1', body: { quality: 4 } })
    const res = await POST(req, makeParams('card-1'))
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(data).toEqual(updated)
  })

  it('forwards Authorization header', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } }),
      )
    })

    const { POST } = await import('./route')
    const req = makeRequest({ token: MOCK_TOKEN })
    await POST(req, makeParams('card-1'))

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('makes request without auth when no token', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } }),
      )
    })

    const { POST } = await import('./route')
    const req = makeRequest({ token: null })
    await POST(req, makeParams('card-1'))

    expect(capturedHeaders['Authorization']).toBeUndefined()
  })

  it('propagates upstream 404 when card not found', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'card not found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const req = makeRequest({ cardId: 'nonexistent' })
    const res = await POST(req, makeParams('nonexistent'))

    expect(res.status).toBe(404)
  })

  it('forwards tl_token cookie as Bearer', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } }),
      )
    })

    const { POST } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/card-1/review', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Cookie: `tl_token=${MOCK_TOKEN}`,
      },
      body: JSON.stringify({ quality: 3 }),
    })
    await POST(req, makeParams('card-1'))

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })
})
