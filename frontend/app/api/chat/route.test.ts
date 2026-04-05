/**
 * Tests for POST /api/chat
 * Falls back to mock stream when TUTORING_SERVICE_URL is unset.
 * Proxies to tutoring service when TUTORING_SERVICE_URL is set.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'chat-test-token'

function makeRequest(opts: {
  messages?: Array<{ role: string; content: string }>
  token?: string
  authHeader?: string
}): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.token) {
    headers['Cookie'] = `tl_token=${opts.token}`
  }
  if (opts.authHeader) {
    headers['Authorization'] = opts.authHeader
  }
  return new NextRequest('http://localhost/api/chat', {
    method: 'POST',
    headers,
    body: JSON.stringify({
      messages: opts.messages ?? [{ role: 'user', content: 'Hello' }],
    }),
  })
}

describe('POST /api/chat — mock stream (no TUTORING_SERVICE_URL)', () => {
  const originalEnv = process.env.TUTORING_SERVICE_URL

  beforeEach(() => {
    delete process.env.TUTORING_SERVICE_URL
    jest.resetModules()
  })

  afterEach(() => {
    if (originalEnv !== undefined) {
      process.env.TUTORING_SERVICE_URL = originalEnv
    } else {
      delete process.env.TUTORING_SERVICE_URL
    }
  })

  it('returns a streaming response with correct Content-Type', async () => {
    const { POST } = await import('./route')
    const res = await POST(makeRequest({}))

    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toBe('text/plain; charset=utf-8')
  })

  it('sets X-Content-Type-Options: nosniff on mock stream', async () => {
    const { POST } = await import('./route')
    const res = await POST(makeRequest({}))

    expect(res.headers.get('X-Content-Type-Options')).toBe('nosniff')
  })

  it('returns a readable body stream', async () => {
    const { POST } = await import('./route')
    const res = await POST(makeRequest({}))

    expect(res.body).not.toBeNull()
  })
})

describe('POST /api/chat — tutoring service proxy', () => {
  const originalFetch = global.fetch
  const originalEnv = process.env.TUTORING_SERVICE_URL

  beforeEach(() => {
    process.env.TUTORING_SERVICE_URL = 'http://tutoring-service:8082'
    jest.resetModules()
  })

  afterEach(() => {
    global.fetch = originalFetch
    if (originalEnv !== undefined) {
      process.env.TUTORING_SERVICE_URL = originalEnv
    } else {
      delete process.env.TUTORING_SERVICE_URL
    }
  })

  it('proxies to tutoring service when TUTORING_SERVICE_URL is set', async () => {
    let capturedUrl = ''
    const fakeStream = new ReadableStream({
      start(c) {
        c.enqueue(new TextEncoder().encode('response chunk'))
        c.close()
      },
    })
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(new Response(fakeStream, { status: 200 }))
    })

    const { POST } = await import('./route')
    const res = await POST(makeRequest({ token: MOCK_TOKEN }))

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain('/v1/chat')
  })

  it('forwards Authorization from cookie to tutoring service', async () => {
    let capturedHeaders: Record<string, string> = {}
    const fakeStream = new ReadableStream({
      start(c) {
        c.close()
      },
    })
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(new Response(fakeStream, { status: 200 }))
    })

    const { POST } = await import('./route')
    await POST(makeRequest({ token: MOCK_TOKEN }))

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('falls back to mock stream when upstream returns non-ok status', async () => {
    // INTENTIONAL Phase 1 behaviour: when TUTORING_SERVICE_URL is set but the
    // upstream returns a non-ok status (e.g. 500 during a deploy), the route
    // silently falls through to the built-in mock stream so the UI stays
    // functional. This is a product decision — see route.ts comment
    // "Fall through to mock". If the product requirement changes to surface the
    // upstream error, replace the fallback with a 502 propagation here.
    global.fetch = jest.fn().mockResolvedValue(new Response('error', { status: 500 }))

    const { POST } = await import('./route')
    const res = await POST(makeRequest({}))

    // Falls through to mock — still returns 200 with streaming body
    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toContain('text/plain; charset=utf-8')
  })
})
