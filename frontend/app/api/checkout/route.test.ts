/**
 * Tests for POST /api/checkout
 * Mocks fetch to user-service, verifies plan forwarding and auth guards.
 */
import { NextRequest } from 'next/server'

// JWT payload: { sub: 'user-123', sub_status: 'trialing' }
const MOCK_TOKEN = [
  'header',
  Buffer.from(JSON.stringify({ sub: 'user-123', sub_status: 'trialing' })).toString('base64url'),
  'sig',
].join('.')

function makeRequest(opts: { body?: unknown; token?: string | null }): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  return new NextRequest('http://localhost/api/checkout', {
    method: 'POST',
    headers,
    body: JSON.stringify(opts.body ?? { planId: 'monthly' }),
  })
}

describe('POST /api/checkout', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('returns 401 when no tl_token cookie', async () => {
    const { POST } = await import('./route')
    const req = makeRequest({ token: null })
    const res = await POST(req)
    expect(res.status).toBe(401)
  })

  it('returns 400 for invalid planId', async () => {
    const { POST } = await import('./route')
    const req = makeRequest({ body: { planId: 'evil' } })
    const res = await POST(req)
    expect(res.status).toBe(400)
  })

  it('forwards correct planId to user-service as plan_id', async () => {
    let capturedBody: unknown
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedBody = JSON.parse(init.body as string)
      return Promise.resolve(
        new Response(
          JSON.stringify({ checkout_url: 'https://checkout.stripe.com/pay/cs_test_123' }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )
    })

    const { POST } = await import('./route')
    const req = makeRequest({ body: { planId: 'semesterly' } })
    const res = await POST(req)
    const data = await res.json()

    expect(res.status).toBe(200)
    expect((capturedBody as { plan_id: string }).plan_id).toBe('semesterly')
    expect(data.checkoutUrl).toBe('https://checkout.stripe.com/pay/cs_test_123')
  })

  it('forwards Authorization Bearer token to user-service', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ checkout_url: 'https://stripe.com/pay/x' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    const req = makeRequest({ body: { planId: 'quarterly' } })
    await POST(req)

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(new Response('Not found', { status: 404 }))

    const { POST } = await import('./route')
    const req = makeRequest({ body: { planId: 'monthly' } })
    const res = await POST(req)
    expect(res.status).toBe(404)
  })
})
