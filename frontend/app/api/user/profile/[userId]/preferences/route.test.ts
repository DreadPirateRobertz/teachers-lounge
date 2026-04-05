/**
 * Tests for PATCH /api/user/profile/[userId]/preferences
 * Proxies to user-service /users/{userId}/preferences with auth forwarding.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'prefs-test-token'

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
  return new NextRequest('http://localhost/api/user/profile/user-77/preferences', {
    method: 'PATCH',
    headers,
    body: JSON.stringify(opts.body ?? { theme: 'dark' }),
  })
}

function makeParams(userId: string) {
  return { params: Promise.resolve({ userId }) }
}

describe('PATCH /api/user/profile/[userId]/preferences', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('proxies PATCH to user-service /users/{userId}/preferences', async () => {
    let capturedUrl = ''
    let capturedBody: unknown
    global.fetch = jest.fn().mockImplementation((url: string, init: RequestInit) => {
      capturedUrl = url
      capturedBody = JSON.parse(init.body as string)
      return Promise.resolve(
        new Response(JSON.stringify({ theme: 'dark' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { PATCH } = await import('./route')
    const res = await PATCH(makeRequest({ body: { theme: 'dark' } }), makeParams('user-77'))
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain('/users/user-77/preferences')
    expect((capturedBody as { theme: string }).theme).toBe('dark')
    expect(data.theme).toBe('dark')
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

    const { PATCH } = await import('./route')
    await PATCH(makeRequest({ token: MOCK_TOKEN }), makeParams('user-77'))

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

    const { PATCH } = await import('./route')
    await PATCH(
      makeRequest({ token: null, authHeader: 'Bearer explicit-override' }),
      makeParams('user-77'),
    )

    expect(capturedHeaders['Authorization']).toBe('Bearer explicit-override')
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

    const { PATCH } = await import('./route')
    await PATCH(makeRequest({ token: null }), makeParams('user-77'))

    expect(capturedHeaders['Authorization']).toBeUndefined()
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'forbidden' }), {
        status: 403,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { PATCH } = await import('./route')
    const res = await PATCH(makeRequest({}), makeParams('user-77'))
    expect(res.status).toBe(403)
  })
})
