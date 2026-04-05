/**
 * Tests for GET /api/flashcards/due
 * Mocks upstream fetch to gaming-service, verifies auth forwarding and status propagation.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'test-token-due'
const GAMING_URL = 'http://gaming-service:8083'

describe('GET /api/flashcards/due', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('returns due cards from gaming-service', async () => {
    const dueCards = [{ id: 'c1', front: 'Q', due_at: '2026-04-05T00:00:00Z' }]
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify(dueCards), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/due', {
      method: 'GET',
      headers: { Authorization: `Bearer ${MOCK_TOKEN}` },
    })
    const res = await GET(req)
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(data).toEqual(dueCards)
    expect((global.fetch as jest.Mock).mock.calls[0][0]).toBe(`${GAMING_URL}/gaming/flashcards/due`)
  })

  it('forwards Authorization header', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify([]), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/due', {
      method: 'GET',
      headers: { Authorization: `Bearer ${MOCK_TOKEN}` },
    })
    await GET(req)

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('forwards tl_token cookie as Bearer', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify([]), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/due', {
      method: 'GET',
      headers: { Cookie: `tl_token=${MOCK_TOKEN}` },
    })
    await GET(req)

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('makes request without auth header when no token present', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify([]), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/due', { method: 'GET' })
    await GET(req)

    expect(capturedHeaders['Authorization']).toBeUndefined()
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'unauthorized' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/due', { method: 'GET' })
    const res = await GET(req)

    expect(res.status).toBe(401)
  })
})
