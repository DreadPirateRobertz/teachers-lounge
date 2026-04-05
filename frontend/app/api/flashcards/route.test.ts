/**
 * Tests for GET /api/flashcards and POST /api/flashcards
 * Mocks upstream fetch to gaming-service, verifies auth forwarding and status propagation.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'test-token-abc'
const GAMING_URL = 'http://gaming-service:8083'

function makeGetRequest(opts: { token?: string | null } = {}): NextRequest {
  const headers: Record<string, string> = {}
  if (opts.token !== null && opts.token !== undefined) {
    headers['Authorization'] = `Bearer ${opts.token}`
  }
  return new NextRequest('http://localhost/api/flashcards', { method: 'GET', headers })
}

function makePostRequest(opts: { body?: unknown; token?: string | null } = {}): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.token !== null && opts.token !== undefined) {
    headers['Authorization'] = `Bearer ${opts.token ?? MOCK_TOKEN}`
  } else if (opts.token === undefined) {
    headers['Cookie'] = `tl_token=${MOCK_TOKEN}`
  }
  return new NextRequest('http://localhost/api/flashcards', {
    method: 'POST',
    headers,
    body: JSON.stringify(opts.body ?? { session_id: 'sess-1' }),
  })
}

describe('GET /api/flashcards', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('proxies cards list from gaming-service', async () => {
    const cards = [{ id: 'c1', front: 'Q', back: 'A' }]
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify(cards), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { GET } = await import('./route')
    const req = makeGetRequest({ token: MOCK_TOKEN })
    const res = await GET(req)
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(data).toEqual(cards)
    expect((global.fetch as jest.Mock).mock.calls[0][0]).toBe(
      `${GAMING_URL}/gaming/flashcards`,
    )
  })

  it('forwards Authorization header to gaming-service', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify([]), { status: 200, headers: { 'Content-Type': 'application/json' } }),
      )
    })

    const { GET } = await import('./route')
    const req = makeGetRequest({ token: MOCK_TOKEN })
    await GET(req)

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('makes request without Authorization when no token', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify([]), { status: 200, headers: { 'Content-Type': 'application/json' } }),
      )
    })

    const { GET } = await import('./route')
    const req = makeGetRequest({ token: null })
    await GET(req)

    expect(capturedHeaders['Authorization']).toBeUndefined()
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'unauthorized' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { GET } = await import('./route')
    const req = makeGetRequest({ token: 'bad-token' })
    const res = await GET(req)

    expect(res.status).toBe(401)
  })

  it('forwards tl_token cookie as Bearer when no Authorization header', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify([]), { status: 200, headers: { 'Content-Type': 'application/json' } }),
      )
    })

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards', {
      method: 'GET',
      headers: { Cookie: `tl_token=${MOCK_TOKEN}` },
    })
    await GET(req)

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })
})

describe('POST /api/flashcards', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('proxies generate request to gaming-service', async () => {
    const generated = [{ id: 'c1' }]
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify(generated), {
          status: 201,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    const req = makePostRequest({ body: { session_id: 'sess-1' } })
    const res = await POST(req)
    const data = await res.json()

    expect(res.status).toBe(201)
    expect(data).toEqual(generated)
    expect(capturedUrl).toBe(`${GAMING_URL}/gaming/flashcards/generate`)
  })

  it('forwards Authorization header on POST', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify([]), { status: 200, headers: { 'Content-Type': 'application/json' } }),
      )
    })

    const { POST } = await import('./route')
    const req = makePostRequest({ token: MOCK_TOKEN })
    await POST(req)

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('propagates upstream error on POST', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'not found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const req = makePostRequest({ body: { session_id: 'missing' } })
    const res = await POST(req)

    expect(res.status).toBe(404)
  })
})
