/**
 * Tests for GET /api/gaming/leaderboard/friends
 * Requires tl_token cookie; proxies with friends query param.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'friends-leaderboard-token'

function makeRequest(opts: { token?: string | null; friends?: string }): NextRequest {
  const headers: Record<string, string> = {}
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  const url = `http://localhost/api/gaming/leaderboard/friends${
    opts.friends ? `?friends=${opts.friends}` : ''
  }`
  return new NextRequest(url, { method: 'GET', headers })
}

describe('GET /api/gaming/leaderboard/friends', () => {
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

  it('proxies to gaming-service friends leaderboard endpoint', async () => {
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
    await GET(makeRequest({ friends: 'u1,u2' }))

    expect(capturedUrl).toContain('/gaming/leaderboard/friends')
    expect(capturedUrl).toContain('friends=u1%2Cu2')
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
      new Response(JSON.stringify({ error: 'not found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { GET } = await import('./route')
    const res = await GET(makeRequest({}))
    expect(res.status).toBe(404)
  })
})
