/**
 * Tests for GET/POST /api/assessment/sessions/[sessionId]
 * GET  → GET  /gaming/assessment/sessions/{id}
 * POST → POST /gaming/assessment/sessions/{id}/answer
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'session-test-token'

function makeRequest(opts: {
  method: 'GET' | 'POST'
  token?: string | null
  authHeader?: string
  body?: unknown
}): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  if (opts.authHeader) {
    headers['Authorization'] = opts.authHeader
  }
  return new NextRequest('http://localhost/api/assessment/sessions/sess-42', {
    method: opts.method,
    headers,
    ...(opts.body != null ? { body: JSON.stringify(opts.body) } : {}),
  })
}

function makeParams(sessionId: string) {
  return { params: Promise.resolve({ sessionId }) }
}

describe('GET /api/assessment/sessions/[sessionId]', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('proxies GET to gaming-service sessions endpoint', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ session_id: 'sess-42', status: 'in_progress' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    const res = await GET(makeRequest({ method: 'GET' }), makeParams('sess-42'))
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/assessment/sessions/sess-42')
    expect(data.session_id).toBe('sess-42')
  })

  it('forwards Authorization from cookie on GET', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({}), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    await GET(makeRequest({ method: 'GET', token: MOCK_TOKEN }), makeParams('sess-42'))

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('GET without auth — no Authorization header forwarded', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({}), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    await GET(makeRequest({ method: 'GET', token: null }), makeParams('sess-42'))

    expect(capturedHeaders['Authorization']).toBeUndefined()
  })

  it('propagates upstream error status on GET', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'not found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { GET } = await import('./route')
    const res = await GET(makeRequest({ method: 'GET' }), makeParams('sess-42'))
    expect(res.status).toBe(404)
  })
})

describe('POST /api/assessment/sessions/[sessionId]', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('proxies POST to .../answer endpoint', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ correct: true }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    const res = await POST(
      makeRequest({ method: 'POST', body: { answer: 'A' } }),
      makeParams('sess-42'),
    )
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/assessment/sessions/sess-42/answer')
    expect(data.correct).toBe(true)
  })

  it('forwards Authorization header on POST', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({}), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    await POST(
      makeRequest({ method: 'POST', token: MOCK_TOKEN }),
      makeParams('sess-42'),
    )

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('propagates upstream error status on POST', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'unprocessable' }), {
        status: 422,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const res = await POST(
      makeRequest({ method: 'POST', body: {} }),
      makeParams('sess-42'),
    )
    expect(res.status).toBe(422)
  })
})
