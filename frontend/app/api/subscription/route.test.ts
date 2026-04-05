/**
 * Tests for GET /api/subscription
 * Requires tl_token cookie; proxies to user-service /users/{userId}/subscription.
 */
import { NextRequest } from 'next/server'

// JWT payload: { sub: 'user-abc' }
const MOCK_TOKEN = [
  'header',
  Buffer.from(JSON.stringify({ sub: 'user-abc' })).toString('base64url'),
  'sig',
].join('.')

function makeRequest(opts: { token?: string | null }): NextRequest {
  const headers: Record<string, string> = {}
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  return new NextRequest('http://localhost/api/subscription', { method: 'GET', headers })
}

describe('GET /api/subscription', () => {
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

  it('returns 401 when token has no sub claim', async () => {
    const tokenNoSub = ['h', Buffer.from(JSON.stringify({})).toString('base64url'), 's'].join('.')
    const { GET } = await import('./route')
    const res = await GET(makeRequest({ token: tokenNoSub }))
    expect(res.status).toBe(401)
  })

  it('proxies to user-service with userId from JWT', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ plan: 'monthly', status: 'active' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    const res = await GET(makeRequest({ token: MOCK_TOKEN }))
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain('/users/user-abc/subscription')
    expect(data.plan).toBe('monthly')
  })

  it('forwards Bearer token to user-service', async () => {
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
    await GET(makeRequest({ token: MOCK_TOKEN }))

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
