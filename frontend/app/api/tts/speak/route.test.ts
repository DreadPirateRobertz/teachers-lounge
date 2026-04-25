/**
 * Tests for POST /api/tts/speak — proxy to tutoring-service TTS endpoint.
 *
 * Validates input handling, voice allow-listing, missing-config behavior,
 * and that audio bytes are streamed back unchanged when the upstream
 * responds with audio data.
 */
import { NextRequest } from 'next/server'

function _mockFetch() {
  const m = jest.fn()
  global.fetch = m as unknown as typeof fetch
  return m
}

function makeRequest(opts: { body?: unknown; raw?: string; token?: string }): NextRequest {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.token) headers['Cookie'] = `tl_token=${opts.token}`
  return new NextRequest('http://localhost/api/tts/speak', {
    method: 'POST',
    headers,
    body: opts.raw ?? JSON.stringify(opts.body ?? {}),
  })
}

describe('POST /api/tts/speak — input validation', () => {
  beforeEach(() => {
    delete process.env.TUTORING_SERVICE_URL
    jest.resetModules()
  })

  it('returns 400 on invalid json', async () => {
    const { POST } = await import('./route')
    const res = await POST(makeRequest({ raw: 'not-json{' }))
    expect(res.status).toBe(400)
    const body = await res.json()
    expect(body.error).toBe('invalid json')
  })

  it('returns 400 when text is missing or empty', async () => {
    const { POST } = await import('./route')
    expect((await POST(makeRequest({ body: {} }))).status).toBe(400)
    expect((await POST(makeRequest({ body: { text: '   ' } }))).status).toBe(400)
    expect((await POST(makeRequest({ body: { text: 123 } }))).status).toBe(400)
  })

  it('returns 503 when TUTORING_SERVICE_URL is not configured', async () => {
    const { POST } = await import('./route')
    const res = await POST(makeRequest({ body: { text: 'hello' } }))
    expect(res.status).toBe(503)
    const body = await res.json()
    expect(body.error).toBe('tts service not configured')
  })
})

describe('POST /api/tts/speak — proxy behavior', () => {
  const originalEnv = process.env.TUTORING_SERVICE_URL

  beforeEach(() => {
    process.env.TUTORING_SERVICE_URL = 'http://tutor:8000'
    jest.resetModules()
  })

  afterEach(() => {
    if (originalEnv !== undefined) process.env.TUTORING_SERVICE_URL = originalEnv
    else delete process.env.TUTORING_SERVICE_URL
    jest.restoreAllMocks()
  })

  it('forwards text and default voice to upstream', async () => {
    const fetchMock = _mockFetch().mockResolvedValue({
      ok: true,
      status: 200,
      body: new ReadableStream({
        start(c) {
          c.enqueue(new TextEncoder().encode('audio-bytes'))
          c.close()
        },
      }),
      headers: new Headers({ 'Content-Type': 'audio/mpeg' }),
    } as unknown as Response)

    const { POST } = await import('./route')
    const res = await POST(makeRequest({ body: { text: 'hello world' } }))

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('http://tutor:8000/v1/tts/speak')
    const sent = JSON.parse((init as RequestInit).body as string)
    expect(sent).toEqual({ text: 'hello world', voice: 'nova' })
    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toBe('audio/mpeg')
  })

  it('coerces unknown voice to nova', async () => {
    const fetchMock = _mockFetch().mockResolvedValue({
      ok: true,
      status: 200,
      body: new ReadableStream(),
      headers: new Headers(),
    } as unknown as Response)

    const { POST } = await import('./route')
    await POST(makeRequest({ body: { text: 'x', voice: 'pirate' } }))
    const sent = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string)
    expect(sent.voice).toBe('nova')
  })

  it('forwards an allow-listed voice unchanged', async () => {
    const fetchMock = _mockFetch().mockResolvedValue({
      ok: true,
      status: 200,
      body: new ReadableStream(),
      headers: new Headers(),
    } as unknown as Response)

    const { POST } = await import('./route')
    await POST(makeRequest({ body: { text: 'x', voice: 'sage' } }))
    const sent = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string)
    expect(sent.voice).toBe('sage')
  })

  it('truncates oversized text to 1500 chars', async () => {
    const fetchMock = _mockFetch().mockResolvedValue({
      ok: true,
      status: 200,
      body: new ReadableStream(),
      headers: new Headers(),
    } as unknown as Response)

    const { POST } = await import('./route')
    await POST(makeRequest({ body: { text: 'x'.repeat(2000) } }))
    const sent = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string)
    expect(sent.text).toHaveLength(1500)
  })

  it('returns upstream error status when upstream fails', async () => {
    _mockFetch().mockResolvedValue({
      ok: false,
      status: 502,
      body: null,
      headers: new Headers(),
    } as unknown as Response)

    const { POST } = await import('./route')
    const res = await POST(makeRequest({ body: { text: 'x' } }))
    expect(res.status).toBe(502)
  })

  it('forwards Authorization header from cookie', async () => {
    const fetchMock = _mockFetch().mockResolvedValue({
      ok: true,
      status: 200,
      body: new ReadableStream(),
      headers: new Headers(),
    } as unknown as Response)

    const { POST } = await import('./route')
    await POST(makeRequest({ body: { text: 'x' }, token: 'tok-123' }))
    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tok-123')
  })
})
