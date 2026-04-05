/**
 * Tests for GET/POST /api/gaming/quests
 * Falls back to mock data when GAMING_SERVICE_URL is not set.
 * Proxies to gaming-service when GAMING_SERVICE_URL is set.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'quests-test-token'

function makeGetRequest(opts: { token?: string } = {}): NextRequest {
  const headers: Record<string, string> = {}
  if (opts.token ?? MOCK_TOKEN) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  return new NextRequest('http://localhost/api/gaming/quests', { method: 'GET', headers })
}

function makePostRequest(opts: { body?: unknown; token?: string } = {}): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  return new NextRequest('http://localhost/api/gaming/quests', {
    method: 'POST',
    headers,
    body: JSON.stringify(opts.body ?? { action: 'questions_answered' }),
  })
}

describe('GET /api/gaming/quests — mock fallback (no GAMING_SERVICE_URL)', () => {
  const originalEnv = process.env.GAMING_SERVICE_URL

  beforeEach(() => {
    delete process.env.GAMING_SERVICE_URL
    jest.resetModules()
  })

  afterEach(() => {
    if (originalEnv !== undefined) {
      process.env.GAMING_SERVICE_URL = originalEnv
    } else {
      delete process.env.GAMING_SERVICE_URL
    }
  })

  it('returns 200 with mock quest data', async () => {
    const { GET } = await import('./route')
    const res = await GET(makeGetRequest())
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(Array.isArray(data.quests)).toBe(true)
    expect(data.quests.length).toBeGreaterThan(0)
  })

  it('mock quests have required shape', async () => {
    const { GET } = await import('./route')
    const res = await GET(makeGetRequest())
    const data = await res.json()
    const quest = data.quests[0]

    expect(quest).toHaveProperty('id')
    expect(quest).toHaveProperty('title')
    expect(quest).toHaveProperty('progress')
    expect(quest).toHaveProperty('target')
    expect(quest).toHaveProperty('xp_reward')
  })
})

describe('POST /api/gaming/quests — mock fallback (no GAMING_SERVICE_URL)', () => {
  const originalEnv = process.env.GAMING_SERVICE_URL

  beforeEach(() => {
    delete process.env.GAMING_SERVICE_URL
    jest.resetModules()
  })

  afterEach(() => {
    if (originalEnv !== undefined) {
      process.env.GAMING_SERVICE_URL = originalEnv
    } else {
      delete process.env.GAMING_SERVICE_URL
    }
  })

  it('returns 200 with mock progress response', async () => {
    const { POST } = await import('./route')
    const res = await POST(makePostRequest())
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(Array.isArray(data.quests)).toBe(true)
    expect(data).toHaveProperty('xp_awarded')
    expect(data).toHaveProperty('gems_awarded')
  })
})

describe('GET /api/gaming/quests — gaming-service proxy', () => {
  const originalFetch = global.fetch
  const originalEnv = process.env.GAMING_SERVICE_URL

  beforeEach(() => {
    process.env.GAMING_SERVICE_URL = 'http://gaming-service:8083'
    jest.resetModules()
  })

  afterEach(() => {
    global.fetch = originalFetch
    if (originalEnv !== undefined) {
      process.env.GAMING_SERVICE_URL = originalEnv
    } else {
      delete process.env.GAMING_SERVICE_URL
    }
  })

  it('proxies GET to gaming-service /gaming/quests/daily', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ quests: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { GET } = await import('./route')
    const res = await GET(makeGetRequest())
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/quests/daily')
    expect(data.quests).toBeDefined()
  })

  it('returns error response when upstream GET fails', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response('service down', { status: 503 }),
    )

    const { GET } = await import('./route')
    const res = await GET(makeGetRequest())

    expect(res.status).toBe(503)
  })
})

describe('POST /api/gaming/quests — gaming-service proxy', () => {
  const originalFetch = global.fetch
  const originalEnv = process.env.GAMING_SERVICE_URL

  beforeEach(() => {
    process.env.GAMING_SERVICE_URL = 'http://gaming-service:8083'
    jest.resetModules()
  })

  afterEach(() => {
    global.fetch = originalFetch
    if (originalEnv !== undefined) {
      process.env.GAMING_SERVICE_URL = originalEnv
    } else {
      delete process.env.GAMING_SERVICE_URL
    }
  })

  it('proxies POST to gaming-service /gaming/quests/progress', async () => {
    let capturedUrl = ''
    global.fetch = jest.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve(
        new Response(JSON.stringify({ quests: [], xp_awarded: 25, gems_awarded: 5 }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    const res = await POST(makePostRequest({ body: { action: 'questions_answered' } }))
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain('/gaming/quests/progress')
    expect(data.xp_awarded).toBe(25)
  })

  it('returns error response when upstream POST fails', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response('bad request', { status: 400 }),
    )

    const { POST } = await import('./route')
    const res = await POST(makePostRequest())

    expect(res.status).toBe(400)
  })
})
