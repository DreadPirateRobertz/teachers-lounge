/**
 * @jest-environment node
 * @fileoverview Tests for the /api/gaming/shop Next.js route handlers.
 *
 * Covers mock-mode fallback (no GAMING_SERVICE_URL) and upstream proxy
 * behavior for GET catalog and POST purchase endpoints.
 */

import { NextRequest } from 'next/server'
import { GET, POST } from './route'

const originalEnv = process.env.GAMING_SERVICE_URL
const originalFetch = global.fetch

beforeEach(() => {
  delete process.env.GAMING_SERVICE_URL
})

afterEach(() => {
  global.fetch = originalFetch
  if (originalEnv !== undefined) {
    process.env.GAMING_SERVICE_URL = originalEnv
  } else {
    delete process.env.GAMING_SERVICE_URL
  }
})

// ── GET /api/gaming/shop ──────────────────────────────────────────────────────

describe('GET /api/gaming/shop', () => {
  it('returns 200 with mock catalog when GAMING_SERVICE_URL is unset', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await GET(req)
    expect(resp.status).toBe(200)
  })

  it('catalog contains exactly 4 items', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await GET(req)
    const data = await resp.json()
    expect(data.items).toHaveLength(4)
  })

  it('every catalog item has a positive gem_cost', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await GET(req)
    const data = await resp.json()
    for (const item of data.items) {
      expect(item.gem_cost).toBeGreaterThan(0)
    }
  })

  it('every catalog item has type, label, icon, and description', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await GET(req)
    const data = await resp.json()
    for (const item of data.items) {
      expect(item.type).toBeTruthy()
      expect(item.label).toBeTruthy()
      expect(item.icon).toBeTruthy()
      expect(item.description).toBeTruthy()
    }
  })
})

// ── POST /api/gaming/shop ─────────────────────────────────────────────────────

describe('POST /api/gaming/shop', () => {
  it('returns 200 with purchase response shape for mock mode', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop', {
      method: 'POST',
      body: JSON.stringify({ user_id: 'u1', power_up: 'shield' }),
      headers: { 'Content-Type': 'application/json' },
    })
    const resp = await POST(req)
    expect(resp.status).toBe(200)
    const data = await resp.json()
    expect(data.power_up).toBe('shield')
    expect(typeof data.gems_left).toBe('number')
    expect(typeof data.new_count).toBe('number')
  })

  it('echoes back the requested power_up type', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop', {
      method: 'POST',
      body: JSON.stringify({ user_id: 'u1', power_up: 'critical' }),
      headers: { 'Content-Type': 'application/json' },
    })
    const resp = await POST(req)
    const data = await resp.json()
    expect(data.power_up).toBe('critical')
  })
})

// ── GET /api/gaming/shop — upstream proxy ─────────────────────────────────────

describe('GET /api/gaming/shop — gaming-service proxy', () => {
  beforeEach(() => {
    process.env.GAMING_SERVICE_URL = 'http://gaming-service:8083'
    jest.resetModules()
  })

  it('proxies GET to gaming-service /gaming/shop/catalog', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ items: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET: getProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await getProxy(req)

    expect(resp.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/shop/catalog')
  })

  it('returns upstream error status when GET fails', async () => {
    global.fetch = jest.fn().mockResolvedValue(new Response('upstream error', { status: 502 }))

    const { GET: getProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await getProxy(req)

    expect(resp.status).toBe(502)
    const data = await resp.json()
    expect(data.error).toBeDefined()
  })

  it('forwards Authorization header from tl_token cookie for GET', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(new Response(JSON.stringify({ items: [] }), { status: 200 }))
    })

    const { GET: getProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/shop', {
      headers: { Cookie: 'tl_token=shop-cookie-token' },
    })
    await getProxy(req)

    expect(capturedHeaders['Authorization']).toBe('Bearer shop-cookie-token')
  })
})

// ── POST /api/gaming/shop — upstream proxy ────────────────────────────────────

describe('POST /api/gaming/shop — gaming-service proxy', () => {
  beforeEach(() => {
    process.env.GAMING_SERVICE_URL = 'http://gaming-service:8083'
    jest.resetModules()
  })

  it('proxies POST to gaming-service /gaming/shop/buy', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ power_up: 'shield', new_count: 1, gems_left: 7 }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST: postProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/shop', {
      method: 'POST',
      body: JSON.stringify({ user_id: 'u1', power_up: 'shield' }),
      headers: { 'Content-Type': 'application/json' },
    })
    const resp = await postProxy(req)
    const data = await resp.json()

    expect(resp.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/shop/buy')
    expect(data.power_up).toBe('shield')
  })

  it('returns upstream error status with text body when POST fails', async () => {
    global.fetch = jest.fn().mockResolvedValue(new Response('insufficient gems', { status: 402 }))

    const { POST: postProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/shop', {
      method: 'POST',
      body: JSON.stringify({ user_id: 'u1', power_up: 'shield' }),
      headers: { 'Content-Type': 'application/json' },
    })
    const resp = await postProxy(req)

    expect(resp.status).toBe(402)
    const data = await resp.json()
    expect(data.error).toBeDefined()
  })
})
