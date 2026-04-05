/**
 * Tests for GET /api/flashcards/export/anki
 * Verifies binary .apkg forwarding, Content-Disposition header, auth forwarding.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'test-token-anki'
const GAMING_URL = 'http://gaming-service:8083'

describe('GET /api/flashcards/export/anki', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
  })

  it('streams binary .apkg content from gaming-service', async () => {
    const fakeApkg = new Uint8Array([0x50, 0x4b, 0x03, 0x04]) // PK zip magic
    global.fetch = jest.fn().mockResolvedValue(
      new Response(fakeApkg.buffer, {
        status: 200,
        headers: {
          'Content-Type': 'application/octet-stream',
          'Content-Disposition': 'attachment; filename="flashcards.apkg"',
        },
      }),
    )

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/export/anki', {
      method: 'GET',
      headers: { Authorization: `Bearer ${MOCK_TOKEN}` },
    })
    const res = await GET(req)

    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toBe('application/octet-stream')
    expect(res.headers.get('Content-Disposition')).toBe('attachment; filename="flashcards.apkg"')
    expect((global.fetch as jest.Mock).mock.calls[0][0]).toBe(
      `${GAMING_URL}/gaming/flashcards/export/anki`,
    )
  })

  it('uses fallback Content-Disposition when upstream does not set it', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(new ArrayBuffer(0), {
        status: 200,
        headers: { 'Content-Type': 'application/octet-stream' },
      }),
    )

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/export/anki', { method: 'GET' })
    const res = await GET(req)

    expect(res.headers.get('Content-Disposition')).toBe('attachment; filename="flashcards.apkg"')
  })

  it('uses fallback Content-Type when upstream does not set it', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(new ArrayBuffer(0), { status: 200, headers: {} }),
    )

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/export/anki', { method: 'GET' })
    const res = await GET(req)

    expect(res.headers.get('Content-Type')).toBe('application/octet-stream')
  })

  it('forwards Authorization header to gaming-service', async () => {
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(new ArrayBuffer(0), {
          status: 200,
          headers: { 'Content-Type': 'application/octet-stream' },
        }),
      )
    })

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/export/anki', {
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
        new Response(new ArrayBuffer(0), {
          status: 200,
          headers: { 'Content-Type': 'application/octet-stream' },
        }),
      )
    })

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/export/anki', {
      method: 'GET',
      headers: { Cookie: `tl_token=${MOCK_TOKEN}` },
    })
    await GET(req)

    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(new ArrayBuffer(0), { status: 500, headers: {} }),
    )

    const { GET } = await import('./route')
    const req = new NextRequest('http://localhost/api/flashcards/export/anki', { method: 'GET' })
    const res = await GET(req)

    expect(res.status).toBe(500)
  })
})
