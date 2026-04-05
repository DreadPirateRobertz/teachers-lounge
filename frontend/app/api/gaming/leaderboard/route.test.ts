/**
 * Tests for GET /api/gaming/leaderboard
 * Requires tl_token cookie; proxies to gaming-service with optional period param.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'leaderboard-token'

function makeRequest(opts: { token?: string | null; period?: string }): NextRequest {
  const headers: Record<string, string> = {}
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  const url = `http://localhost/api/gaming/leaderboard${opts.period ? `?period=${opts.period}` : ''}`
  return new NextRequest(url, { method: 'GET', headers })
}

describe('GET /api/gaming/leaderboard', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('returns 401 when no tl_token cookie', async () => {
    const { GET } = await import('./route')
    const res = await GET(makeRequest({ token: null }))
    expect(res.status).toBe(401)
  })

  it('defaults to all_time period when none provided', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ entries: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    await GET(makeRequest({}))

    expect(capturedUrl).toContain('period=all_time')
  })

  it('forwards explicit period to upstream', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ entries: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    await GET(makeRequest({ period: 'weekly' }))

    expect(capturedUrl).toContain('period=weekly')
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
    await GET(makeRequest({}))

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'server error' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { GET } = await import('./route')
    const res = await GET(makeRequest({}))
    expect(res.status).toBe(500)
  })
})
