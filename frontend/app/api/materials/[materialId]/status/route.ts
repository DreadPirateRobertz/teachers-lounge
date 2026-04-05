import { NextRequest, NextResponse } from 'next/server'

/** UUID v4 pattern for materialId path parameter validation. */
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

/** Processing statuses surfaced by the ingestion service. */
export type MaterialStatus = 'pending' | 'processing' | 'complete' | 'failed'

/** Shape returned by this route on success. */
export interface MaterialStatusResponse {
  material_id: string
  status: MaterialStatus
  chunk_count: number
}

/**
 * GET /api/materials/[materialId]/status
 *
 * Polls the ingestion service for the current processing status of a material.
 * When `INGESTION_SERVICE_URL` is unset (local dev / CI), returns a mock
 * `complete` response so the upload UI can be exercised without the backend.
 *
 * The ingestion service field `processing_status` is mapped to `status` so
 * that the frontend can use a single uniform field name regardless of which
 * service version is behind the proxy.
 *
 * @param req    - Incoming Next.js request.
 * @param params - Route segment params containing `materialId`.
 * @returns JSON `MaterialStatusResponse` on success, or an error JSON with
 *          `{ detail }` and the appropriate HTTP status code.
 */
export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ materialId: string }> },
): Promise<NextResponse> {
  // Next.js 15: params is a Promise — must be awaited before use.
  const { materialId } = await params
  // Read env inside handler so test mutations to process.env take effect.
  const INGESTION_SERVICE_URL = process.env.INGESTION_SERVICE_URL

  if (!UUID_RE.test(materialId)) {
    return NextResponse.json({ detail: 'materialId must be a valid UUID' }, { status: 400 })
  }

  // Dev / CI mock — no ingestion service wired yet.
  if (!INGESTION_SERVICE_URL) {
    const response: MaterialStatusResponse = {
      material_id: materialId,
      status: 'complete',
      chunk_count: 0,
    }
    return NextResponse.json(response)
  }

  const tokenCookie = req.cookies.get('tl_token')?.value
  const authHeader =
    req.headers.get('authorization') ?? (tokenCookie ? `Bearer ${tokenCookie}` : null)

  if (!authHeader) {
    return NextResponse.json({ detail: 'unauthorized' }, { status: 401 })
  }

  let upstream: Response
  try {
    upstream = await fetch(`${INGESTION_SERVICE_URL}/v1/ingest/${materialId}/status`, {
      headers: { Authorization: authHeader },
    })
  } catch {
    return NextResponse.json({ detail: 'ingestion service unavailable' }, { status: 502 })
  }

  const data: unknown = await upstream.json().catch(() => ({ detail: upstream.statusText }))

  // Map ingestion service field `processing_status` → `status`.
  if (
    upstream.ok &&
    typeof data === 'object' &&
    data !== null &&
    'processing_status' in data
  ) {
    const row = data as Record<string, unknown>
    const response: MaterialStatusResponse = {
      material_id: String(row.material_id ?? materialId),
      status: row.processing_status as MaterialStatus,
      chunk_count: typeof row.chunk_count === 'number' ? row.chunk_count : 0,
    }
    return NextResponse.json(response)
  }

  return NextResponse.json(data, { status: upstream.status })
}
