/**
 * Tests for POST /api/assessment
 * Proxies to gaming-service /gaming/assessment/start with auth forwarding.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'test-token-abc'

function makeRequest(opts: {
  body?: unknown
  token?: string | null
  authHeader?: string
}): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  if (opts.authHeader) {
    headers['Authorization'] = opts.authHeader
  }
  return new NextRequest('http://localhost/api/assessment', {
    method: 'POST',
    headers,
    body: JSON.stringify(opts.body ?? { subject: 'math' }),
  })
}

describe('POST /api/assessment', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('proxies request body to gaming-service', async () => {
    let capturedBody: unknown
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedBody = JSON.parse(init.body as string)
      return Promise.resolve(
        new Response(JSON.stringify({ session_id: 's1' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    const res = await POST(makeRequest({ body: { subject: 'science' } }))
    const data = await res.json()

    expect(res.status).toBe(200)
    expect((capturedBody as { subject: string }).subject).toBe('science')
    expect(data.session_id).toBe('s1')
  })

  it('forwards Authorization header from cookie', async () => {
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
    await POST(makeRequest({ token: MOCK_TOKEN }))

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('forwards explicit Authorization header', async () => {
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
    await POST(makeRequest({ token: null, authHeader: 'Bearer explicit-token' }))

    expect(capturedHeaders['Authorization']).toBe('Bearer explicit-token')
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'bad request' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const res = await POST(makeRequest({}))
    expect(res.status).toBe(400)
  })

  it('propagates 401 when upstream rejects unauthenticated request', async () => {
    // The Next.js route is a transparent proxy — it does not enforce auth itself.
    // Auth is enforced by the gaming-service; when no token is present the
    // upstream returns 401 which the route propagates to the client.
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ detail: 'unauthorized' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const res = await POST(makeRequest({ token: null }))

    expect(res.status).toBe(401)
  })

  it('calls gaming-service assessment/start endpoint', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({}), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    await POST(makeRequest({}))

    expect(capturedUrl).toContain('/gaming/assessment/start')
  })
})
