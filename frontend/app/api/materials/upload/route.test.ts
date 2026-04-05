/**
 * Tests for POST /api/materials/upload
 * Requires course_id query param.
 * Falls back to mock 202 when INGESTION_SERVICE_URL is not set.
 * Requires auth when INGESTION_SERVICE_URL is set.
 */
import { NextRequest } from 'next/server'

const MOCK_TOKEN = 'upload-test-token'

function makeRequest(opts: {
  courseId?: string | null
  token?: string | null
  authHeader?: string
  file?: File
}): NextRequest {
  const headers: Record<string, string> = {}
  if (opts.token !== null) {
    headers['Cookie'] = `tl_token=${opts.token ?? MOCK_TOKEN}`
  }
  if (opts.authHeader) {
    headers['Authorization'] = opts.authHeader
  }

  const url = `http://localhost/api/materials/upload${
    opts.courseId !== null && opts.courseId !== undefined ? `?course_id=${opts.courseId}` : ''
  }`

  const formData = new FormData()
  formData.append(
    'file',
    opts.file ?? new File(['content'], 'test.pdf', { type: 'application/pdf' }),
  )

  return new NextRequest(url, { method: 'POST', headers, body: formData })
}

describe('POST /api/materials/upload — validation', () => {
  beforeEach(() => {
    delete process.env.INGESTION_SERVICE_URL
    jest.resetModules()
  })

  afterEach(() => {
    jest.resetModules()
  })

  it('returns 400 when course_id is missing', async () => {
    const { POST } = await import('./route')
    const req = makeRequest({ courseId: null })
    const res = await POST(req)
    const data = await res.json()

    expect(res.status).toBe(400)
    expect(data.detail).toContain('course_id')
  })

  it('returns 400 when course_id is not a valid UUID', async () => {
    const { POST } = await import('./route')
    const req = makeRequest({ courseId: '../etc/passwd' })
    const res = await POST(req)
    const data = await res.json()

    expect(res.status).toBe(400)
    expect(data.detail).toContain('UUID')
  })

  it('accepts a valid UUID course_id and returns 202 from mock', async () => {
    const { POST } = await import('./route')
    const req = makeRequest({ courseId: '550e8400-e29b-41d4-a716-446655440000' })
    const res = await POST(req)
    expect(res.status).toBe(202)
  })

  it('returns 413 when content-length header exceeds 500 MB', async () => {
    const { POST } = await import('./route')
    const headers: Record<string, string> = {
      Cookie: `tl_token=${MOCK_TOKEN}`,
      'Content-Length': String(501 * 1024 * 1024),
    }
    const url = `http://localhost/api/materials/upload?course_id=550e8400-e29b-41d4-a716-446655440000`
    const req = new NextRequest(url, { method: 'POST', headers, body: new FormData() })
    const res = await POST(req)
    expect(res.status).toBe(413)
  })
})

describe('POST /api/materials/upload — mock fallback (no INGESTION_SERVICE_URL)', () => {
  const originalEnv = process.env.INGESTION_SERVICE_URL
  // Valid UUID v4 constants — route enforces UUID format before reaching mock path.
  const MOCK_COURSE_ID = '550e8400-e29b-41d4-a716-446655440000'
  const MOCK_COURSE_ID_2 = '550e8400-e29b-41d4-a716-446655440042'

  beforeEach(() => {
    delete process.env.INGESTION_SERVICE_URL
    jest.resetModules()
  })

  afterEach(() => {
    if (originalEnv !== undefined) {
      process.env.INGESTION_SERVICE_URL = originalEnv
    } else {
      delete process.env.INGESTION_SERVICE_URL
    }
  })

  it('returns 202 with mock job response', async () => {
    const { POST } = await import('./route')
    const res = await POST(makeRequest({ courseId: MOCK_COURSE_ID }))
    const data = await res.json()

    expect(res.status).toBe(202)
    expect(data).toHaveProperty('job_id')
    expect(data).toHaveProperty('material_id')
    expect(data.status).toBe('pending')
  })

  it('mock gcs_path includes courseId and filename', async () => {
    const file = new File(['pdf bytes'], 'lecture.pdf', { type: 'application/pdf' })
    const { POST } = await import('./route')
    const res = await POST(makeRequest({ courseId: MOCK_COURSE_ID_2, file }))
    const data = await res.json()

    expect(data.gcs_path).toContain(MOCK_COURSE_ID_2)
    expect(data.gcs_path).toContain('lecture.pdf')
  })
})

describe('POST /api/materials/upload — ingestion service proxy', () => {
  const originalFetch = global.fetch
  const originalEnv = process.env.INGESTION_SERVICE_URL
  // Valid UUID v4 — route enforces UUID format before proxying to ingestion service.
  const PROXY_COURSE_ID = '550e8400-e29b-41d4-a716-446655440001'

  beforeEach(() => {
    process.env.INGESTION_SERVICE_URL = 'http://ingestion-service:8084'
    jest.resetModules()
  })

  afterEach(() => {
    global.fetch = originalFetch
    if (originalEnv !== undefined) {
      process.env.INGESTION_SERVICE_URL = originalEnv
    } else {
      delete process.env.INGESTION_SERVICE_URL
    }
  })

  it('returns 401 when no auth provided', async () => {
    const { POST } = await import('./route')
    const res = await POST(makeRequest({ courseId: PROXY_COURSE_ID, token: null }))
    const data = await res.json()

    expect(res.status).toBe(401)
    expect(data.detail).toBe('unauthorized')
  })

  it('proxies to ingestion service with course_id and Authorization', async () => {
    let capturedUrl = ''
    let capturedHeaders: Record<string, string> = {}
    global.fetch = jest.fn().mockImplementation((url: string, init: RequestInit) => {
      capturedUrl = url
      capturedHeaders = init.headers as Record<string, string>
      return Promise.resolve(
        new Response(JSON.stringify({ job_id: 'job-x', material_id: 'mat-x', status: 'pending' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })

    const { POST } = await import('./route')
    const res = await POST(makeRequest({ courseId: PROXY_COURSE_ID, token: MOCK_TOKEN }))

    expect(res.status).toBe(200)
    expect(capturedUrl).toContain(`course_id=${PROXY_COURSE_ID}`)
    expect(capturedUrl).toContain('/v1/ingest/upload')
    expect(capturedHeaders['Authorization']).toBe(`Bearer ${MOCK_TOKEN}`)
  })

  it('returns 502 when ingestion service is unreachable', async () => {
    global.fetch = jest.fn().mockRejectedValue(new Error('ECONNREFUSED'))

    const { POST } = await import('./route')
    const res = await POST(makeRequest({ courseId: PROXY_COURSE_ID }))
    const data = await res.json()

    expect(res.status).toBe(502)
    expect(data.detail).toBe('ingestion service unavailable')
  })

  it('propagates upstream error status', async () => {
    global.fetch = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ detail: 'unprocessable entity' }), {
        status: 422,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const { POST } = await import('./route')
    const res = await POST(makeRequest({ courseId: PROXY_COURSE_ID }))
    expect(res.status).toBe(422)
  })
})
