/**
 * @jest-environment node
 * @fileoverview Tests for the /api/gaming/progression Next.js route handler.
 *
 * Covers mock-mode fallback (no GAMING_SERVICE_URL) and upstream proxy
 * behavior including success, error, and auth header forwarding.
 */

import { NextRequest } from 'next/server'
import { GET } from './route'

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

describe('GET /api/gaming/progression', () => {
  it('returns 200 in mock mode', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    expect(resp.status).toBe(200)
  })

  it('response contains nodes array', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    expect(Array.isArray(data.nodes)).toBe(true)
    expect(data.nodes.length).toBeGreaterThan(0)
  })

  it('response contains total_defeated number', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    expect(typeof data.total_defeated).toBe('number')
  })

  it('every node has required fields', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    for (const node of data.nodes) {
      expect(node.boss_id).toBeTruthy()
      expect(node.name).toBeTruthy()
      expect(node.tier).toBeGreaterThan(0)
      expect(['defeated', 'current', 'locked']).toContain(node.state)
      expect(node.primary_color).toMatch(/^#[0-9a-f]{6}$/i)
    }
  })

  it('nodes are ordered by ascending tier', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    for (let i = 1; i < data.nodes.length; i++) {
      expect(data.nodes[i].tier).toBeGreaterThanOrEqual(data.nodes[i - 1].tier)
    }
  })

  it('mock has exactly one current node and rest are defeated or locked', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    const currentNodes = data.nodes.filter((n: { state: string }) => n.state === 'current')
    expect(currentNodes).toHaveLength(1)
  })
})

describe('GET /api/gaming/progression — gaming-service proxy', () => {
  beforeEach(() => {
    process.env.GAMING_SERVICE_URL = 'http://gaming-service:8083'
    jest.resetModules()
  })

  it('proxies GET to gaming-service /gaming/boss/progression', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ total_defeated: 2, nodes: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET: getProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await getProxy(req)
    const data = await resp.json()

    expect(resp.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/boss/progression')
    expect(data.total_defeated).toBe(2)
  })

  it('returns upstream error status when gaming-service fails', async () => {
    global.fetch = jest.fn().mockResolvedValue(new Response('service unavailable', { status: 503 }))

    const { GET: getProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await getProxy(req)

    expect(resp.status).toBe(503)
    const data = await resp.json()
    expect(data.error).toBeDefined()
  })

  it('forwards Authorization header from request header', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ total_defeated: 0, nodes: [] }), { status: 200 }),
      )
    })

    const { GET: getProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/progression', {
      headers: { Authorization: 'Bearer my-token' },
    })
    await getProxy(req)

    expect(capturedHeaders['Authorization']).toBe('Bearer my-token')
  })

  it('forwards auth token from tl_token cookie', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ total_defeated: 0, nodes: [] }), { status: 200 }),
      )
    })

    const { GET: getProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/progression', {
      headers: { Cookie: 'tl_token=cookie-jwt' },
    })
    await getProxy(req)

    expect(capturedHeaders['Authorization']).toBe('Bearer cookie-jwt')
  })

  it('sends no Authorization header when unauthenticated', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedHeaders = opts.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ total_defeated: 0, nodes: [] }), { status: 200 }),
      )
    })

    const { GET: getProxy } = await import('./route')
    const req = new NextRequest('http://localhost/api/gaming/progression')
    await getProxy(req)

    expect(capturedHeaders['Authorization']).toBeUndefined()
  })
})
