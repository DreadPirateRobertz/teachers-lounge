/**
 * Tests for GET /api/user/profile/[userId]
 *
 * Proxies to user-service GET /users/{userId}/profile with auth forwarding.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'profile-test-token'

function makeRequest(opts: {
  token?: string | null
  authHeader?: string
}): NextRequest {
  const headers: Record<string, string> = {}
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  if (opts.authHeader) {
    headers['Authorization'] = opts.authHeader
  }
  return new NextRequest('http://localhost/api/user/profile/user-42', { headers })
}

function makeParams(userId: string) {
  return { params: Promise.resolve({ userId }) }
}

describe('GET /api/user/profile/[userId]', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('proxies GET to user-service /users/{userId}/profile', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ user_id: 'user-42', felder_silverman_dials: {} }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    const res = await GET(makeRequest({}), makeParams('user-42'))
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain('/users/user-42/profile')
    expect(data.user_id).toBe('user-42')
  })

  it('forwards Authorization from cookie', async () => {
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
    await GET(makeRequest({ token: MOCK_TOKEN }), makeParams('user-42'))

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('forwards explicit Authorization header over cookie', async () => {
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
    await GET(
      makeRequest({ token: null, authHeader: 'Bearer explicit-token' }),
      makeParams('user-42'),
    )

    expect(capturedHeaders['Authorization']).toBe('Bearer explicit-token')
  })

  it('sends no Authorization when no token or header provided', async () => {
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
    await GET(makeRequest({ token: null }), makeParams('user-42'))

    expect(capturedHeaders['Authorization']).toBeUndefined()
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'forbidden' }), {
        status: 403,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { GET } = await import('./route')
    const res = await GET(makeRequest({}), makeParams('user-42'))
    expect(res.status).toBe(403)
  })
})
