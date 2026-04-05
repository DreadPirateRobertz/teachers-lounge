/**
 * Tests for GET /api/gaming/achievements/[userId]
 * Requires tl_token cookie; proxies to gaming-service achievements endpoint.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'achievements-test-token'

function makeRequest(opts: { token?: string | null }): NextRequest {
  const headers: Record<string, string> = {}
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  return new NextRequest('http://localhost/api/gaming/achievements/user-999', {
    method: 'GET',
    headers,
  })
}

function makeParams(userId: string) {
  return { params: Promise.resolve({ userId }) }
}

describe('GET /api/gaming/achievements/[userId]', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('returns 401 when no tl_token cookie', async () => {
    const { GET } = await import('./route')
    const res = await GET(makeRequest({ token: null }), makeParams('user-999'))
    expect(res.status).toBe(401)
  })

  it('proxies to gaming-service achievements/{userId}', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ achievements: ['first_login'] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    const res = await GET(makeRequest({}), makeParams('user-999'))
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/achievements/user-999')
    expect(data.achievements).toContain('first_login')
  })

  it('URL-encodes userId in upstream path', async () => {
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
    await GET(makeRequest({}), makeParams('user with spaces'))

    expect(capturedUrl).toContain('user%20with%20spaces')
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
    await GET(makeRequest({}), makeParams('user-999'))

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
    const res = await GET(makeRequest({}), makeParams('user-999'))
    expect(res.status).toBe(404)
  })
})
