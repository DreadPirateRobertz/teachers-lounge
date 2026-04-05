/**
 * Tests for GET /api/analytics/[...path]
 * Requires tl_token cookie; proxies to analytics-service with path + query forwarding.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'analytics-test-token'

function makeRequest(opts: { path?: string; token?: string | null; search?: string }): NextRequest {
  const headers: Record<string, string> = {}
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  const url = `http://localhost/api/analytics/${opts.path ?? 'events'}${opts.search ?? ''}`
  return new NextRequest(url, { method: 'GET', headers })
}

function makeParams(path: string[]) {
  return { params: Promise.resolve({ path }) }
}

describe('GET /api/analytics/[...path]', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('returns 401 when no tl_token cookie', async () => {
    const { GET } = await import('./route')
    const res = await GET(makeRequest({ token: null }), makeParams(['events']))
    expect(res.status).toBe(401)
  })

  it('proxies to analytics-service with correct path', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ events: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    await GET(makeRequest({ path: 'events/click' }), makeParams(['events', 'click']))

    expect(capturedUrl).toContain('/v1/analytics/events/click')
  })

  it('includes query string in upstream URL', async () => {
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

    const { GET } = await import('./route')
    await GET(
      makeRequest({ path: 'events', search: '?from=2026-01-01&limit=10' }),
      makeParams(['events']),
    )

    expect(capturedUrl).toContain('from=2026-01-01')
    expect(capturedUrl).toContain('limit=10')
  })

  it('forwards Authorization Bearer token', async () => {
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
    await GET(makeRequest({}), makeParams(['events']))

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'service unavailable' }), {
        status: 503,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { GET } = await import('./route')
    const res = await GET(makeRequest({}), makeParams(['events']))
    expect(res.status).toBe(503)
  })
})
