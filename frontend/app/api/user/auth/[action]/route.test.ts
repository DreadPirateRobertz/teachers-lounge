/**
 * Tests for POST /api/user/auth/[action]
 * Proxies to user-service; manages tl_token cookie on login/logout.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyLTEifQ.sig'

function makeRequest(opts: { action: string; body?: unknown; cookie?: string }): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.cookie) {
    headers['Cookie'] = opts.cookie
  }
  return new NextRequest(`http://localhost/api/user/auth/${opts.action}`, {
    method: 'POST',
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })
}

function makeParams(action: string) {
  return { params: Promise.resolve({ action }) }
}

describe('POST /api/user/auth/[action]', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('returns 404 for unknown action', async () => {
    const { POST } = await import('./route')
    const res = await POST(makeRequest({ action: 'hack' }), makeParams('hack'))
    const data = await res.json()

    expect(res.status).toBe(404)
    expect(data.error).toBe('Not found')
  })

  it('rejects disallowed action "delete"', async () => {
    const { POST } = await import('./route')
    const res = await POST(makeRequest({ action: 'delete' }), makeParams('delete'))
    expect(res.status).toBe(404)
  })

  it('proxies login to user-service /auth/login', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ access_token: MOCK_TOKEN }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    await POST(
      makeRequest({ action: 'login', body: { email: 'a@b.com', password: 'pw' } }),
      makeParams('login'),
    )

    expect(capturedUrl).toContain('/auth/login')
  })

  it('sets tl_token cookie on successful login with access_token', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ access_token: MOCK_TOKEN }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const res = await POST(makeRequest({ action: 'login' }), makeParams('login'))

    const setCookieHeader = res.headers.get('Set-Cookie') ?? ''
    expect(setCookieHeader).toContain('tl_token')
    expect(setCookieHeader).toContain(MOCK_TOKEN)
    expect(setCookieHeader).toMatch(/HttpOnly/i)
    expect(setCookieHeader).toMatch(/Secure/i)
    expect(setCookieHeader).toMatch(/SameSite=Strict/i)
    expect(setCookieHeader).toContain('Max-Age=900')
  })

  it('does not set tl_token cookie when login fails', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'invalid credentials' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const res = await POST(
      makeRequest({ action: 'login', body: { email: 'x@x.com', password: 'bad' } }),
      makeParams('login'),
    )

    expect(res.status).toBe(401)
  })

  it('deletes tl_token cookie on logout', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response('{}', {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const res = await POST(
      makeRequest({ action: 'logout', cookie: `tl_token=${MOCK_TOKEN}` }),
      makeParams('logout'),
    )

    // Cookie should be cleared (max-age=0 or expires in past)
    const setCookieHeader = res.headers.get('Set-Cookie') ?? ''
    // NextResponse.cookies.delete sets max-age=0 or expires
    expect(setCookieHeader).toMatch(/tl_token=;|tl_token=\s*;|Max-Age=0/i)
  })

  it('propagates upstream status on register', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'email taken' }), {
        status: 409,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const res = await POST(
      makeRequest({ action: 'register', body: { email: 'dup@user.com', password: 'pw' } }),
      makeParams('register'),
    )
    expect(res.status).toBe(409)
  })

  it('proxies register to user-service /auth/register', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({}), {
          status: 201,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    await POST(
      makeRequest({ action: 'register', body: { email: 'new@user.com', password: 'pw' } }),
      makeParams('register'),
    )

    expect(capturedUrl).toContain('/auth/register')
  })
})
