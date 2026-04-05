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

describe('POST /api/chat — input validation', () => {
  beforeEach(() => {
    delete process.env.TUTORING_SERVICE_URL
    jest.resetModules()
  })

  it('rejects requests with more than 50 messages', async () => {
    const { POST } = await import('./route')
    const messages = Array.from({ length: 51 }, (_, i) => ({ role: 'user', content: `msg ${i}` }))
    const req = new NextRequest('http://localhost/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ messages }),
    })
    const res = await POST(req)
    expect(res.status).toBe(400)
    const body = await res.json()
    expect(body.error).toBe('too many messages')
  })

  it('truncates message content exceeding 4000 chars and serves response', async () => {
    const { POST } = await import('./route')
    const longContent = 'x'.repeat(5000)
    const req = new NextRequest('http://localhost/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ messages: [{ role: 'user', content: longContent }] }),
    })
    const res = await POST(req)
    // Should not reject — truncated content falls back to mock stream
    expect(res.status).toBe(200)
  })

  it('handles missing messages array gracefully', async () => {
    const { POST } = await import('./route')
    const req = new NextRequest('http://localhost/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    })
    const res = await POST(req)
    expect(res.status).toBe(200) // Falls through to mock
  })
})

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
    expect(res.headers.get('Content-Type')).toContain('text/plain')
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

  it('falls back to mock stream when upstream is not ok', async () => {
    global.fetch = jest.fn().mockResolvedValue(new Response('error', { status: 500 }))

    const { POST } = await import('./route')
    const res = await POST(makeRequest({}))

    // Falls through to mock — still returns 200 with streaming body
    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toContain('text/plain')
  })
})
