/**
 * @jest-environment node
 *
 * Tests for GET /api/materials/[materialId]/status
 *
 * Covers: UUID validation, mock-mode response, auth enforcement,
 * upstream proxy, processing_status→status field mapping, and error paths.
 */
import { NextRequest } from 'next/server'
import { GET } from './route'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const VALID_UUID = 'a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5'
const INVALID_ID = 'not-a-uuid'

/**
 * Build a minimal NextRequest for a status poll.
 *
 * @param materialId - Path segment value.
 * @param authHeader - Optional Authorization header value.
 * @param tokenCookie - Optional tl_token cookie value.
 * @returns NextRequest pointed at the route URL.
 */
function makeRequest(
  materialId: string,
  authHeader?: string,
  tokenCookie?: string,
): NextRequest {
  const url = `http://localhost/api/materials/${materialId}/status`
  const headers: Record<string, string> = {}
  if (authHeader) headers['authorization'] = authHeader
  if (tokenCookie) headers['cookie'] = `tl_token=${tokenCookie}`
  return new NextRequest(url, { headers })
}

/** Invoke the route handler with the given materialId. */
async function callRoute(
  materialId: string,
  opts: { authHeader?: string; tokenCookie?: string } = {},
) {
  const req = makeRequest(materialId, opts.authHeader, opts.tokenCookie)
  return GET(req, { params: { materialId } })
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('GET /api/materials/[materialId]/status — validation', () => {
  it('returns 400 for a non-UUID materialId', async () => {
    const res = await callRoute(INVALID_ID, { authHeader: 'Bearer tok' })
    expect(res.status).toBe(400)
    const body = await res.json()
    expect(body.detail).toMatch(/valid uuid/i)
  })
})

describe('GET /api/materials/[materialId]/status — mock mode (no INGESTION_SERVICE_URL)', () => {
  beforeEach(() => {
    delete process.env.INGESTION_SERVICE_URL
  })

  it('returns 200 with status=complete for a valid UUID', async () => {
    const res = await callRoute(VALID_UUID)
    expect(res.status).toBe(200)
    const body = await res.json()
    expect(body.status).toBe('complete')
    expect(body.material_id).toBe(VALID_UUID)
    expect(typeof body.chunk_count).toBe('number')
  })

  it('returns mock response without requiring auth', async () => {
    const res = await callRoute(VALID_UUID)
    expect(res.status).toBe(200)
  })
})

describe('GET /api/materials/[materialId]/status — auth enforcement', () => {
  beforeEach(() => {
    process.env.INGESTION_SERVICE_URL = 'http://ingest.svc'
  })
  afterEach(() => {
    delete process.env.INGESTION_SERVICE_URL
  })

  it('returns 401 when no auth header or cookie', async () => {
    const res = await callRoute(VALID_UUID)
    expect(res.status).toBe(401)
  })

  it('accepts Bearer token from Authorization header', async () => {
    global.fetch = jest.fn().mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        material_id: VALID_UUID,
        processing_status: 'complete',
        chunk_count: 5,
      }),
    }) as jest.Mock

    const res = await callRoute(VALID_UUID, { authHeader: 'Bearer mytoken' })
    expect(res.status).toBe(200)
  })

  it('accepts Bearer token from tl_token cookie', async () => {
    global.fetch = jest.fn().mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        material_id: VALID_UUID,
        processing_status: 'complete',
        chunk_count: 0,
      }),
    }) as jest.Mock

    const res = await callRoute(VALID_UUID, { tokenCookie: 'cookie-tok' })
    expect(res.status).toBe(200)
  })
})

describe('GET /api/materials/[materialId]/status — proxy', () => {
  beforeEach(() => {
    process.env.INGESTION_SERVICE_URL = 'http://ingest.svc'
  })
  afterEach(() => {
    delete process.env.INGESTION_SERVICE_URL
    jest.restoreAllMocks()
  })

  it('maps processing_status to status in the response', async () => {
    global.fetch = jest.fn().mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        material_id: VALID_UUID,
        processing_status: 'processing',
        chunk_count: 0,
      }),
    }) as jest.Mock

    const res = await callRoute(VALID_UUID, { authHeader: 'Bearer tok' })
    const body = await res.json()
    expect(body.status).toBe('processing')
    expect(body).not.toHaveProperty('processing_status')
  })

  it('returns chunk_count from upstream', async () => {
    global.fetch = jest.fn().mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        material_id: VALID_UUID,
        processing_status: 'complete',
        chunk_count: 42,
      }),
    }) as jest.Mock

    const res = await callRoute(VALID_UUID, { authHeader: 'Bearer tok' })
    const body = await res.json()
    expect(body.chunk_count).toBe(42)
  })

  it('returns 404 when upstream returns 404', async () => {
    global.fetch = jest.fn().mockResolvedValueOnce({
      ok: false,
      status: 404,
      statusText: 'Not Found',
      json: async () => ({ detail: 'material not found' }),
    }) as jest.Mock

    const res = await callRoute(VALID_UUID, { authHeader: 'Bearer tok' })
    expect(res.status).toBe(404)
  })

  it('returns 502 when the ingestion service is unreachable', async () => {
    global.fetch = jest.fn().mockRejectedValueOnce(new Error('ECONNREFUSED')) as jest.Mock

    const res = await callRoute(VALID_UUID, { authHeader: 'Bearer tok' })
    expect(res.status).toBe(502)
    const body = await res.json()
    expect(body.detail).toMatch(/unavailable/i)
  })

  it('calls the ingestion service with the correct URL', async () => {
    const mockFetch = jest.fn().mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        material_id: VALID_UUID,
        processing_status: 'complete',
        chunk_count: 0,
      }),
    }) as jest.Mock
    global.fetch = mockFetch

    await callRoute(VALID_UUID, { authHeader: 'Bearer tok' })

    expect(mockFetch).toHaveBeenCalledWith(
      `http://ingest.svc/v1/ingest/${VALID_UUID}/status`,
      expect.objectContaining({ headers: { Authorization: 'Bearer tok' } }),
    )
  })
})
