import { NextRequest, NextResponse } from 'next/server'

const INGESTION_SERVICE_URL = process.env.INGESTION_SERVICE_URL

/** Maximum upload size enforced at the Next.js boundary (500 MB). */
const MAX_UPLOAD_BYTES = 500 * 1024 * 1024

/** UUID v4 pattern — validates course_id before forwarding to the ingestion service. */
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

export async function POST(req: NextRequest): Promise<NextResponse> {
  const courseId = req.nextUrl.searchParams.get('course_id')
  if (!courseId) {
    return NextResponse.json({ detail: 'course_id is required' }, { status: 400 })
  }
  // Validate course_id is a UUID before forwarding to avoid path-injection.
  if (!UUID_RE.test(courseId)) {
    return NextResponse.json({ detail: 'course_id must be a valid UUID' }, { status: 400 })
  }

  // Reject oversized requests early before reading the body.
  // Limitation: Content-Length can be omitted (chunked transfer) or spoofed —
  // this is a best-effort early gate only. The authoritative size limit is
  // enforced by the ingestion service and the infrastructure reverse proxy
  // (nginx/GKE Ingress client_max_body_size). Do not rely on this check alone.
  const contentLength = Number(req.headers.get('content-length') ?? 0)
  if (contentLength > MAX_UPLOAD_BYTES) {
    return NextResponse.json({ detail: 'file too large' }, { status: 413 })
  }

  // Phase 1: ingestion service not yet wired — return a mock 202 response so
  // the upload UI can be developed and tested independently.
  if (!INGESTION_SERVICE_URL) {
    const formData = await req.formData()
    const file = formData.get('file')
    const filename = file instanceof File ? file.name : 'upload'
    return NextResponse.json(
      {
        job_id: crypto.randomUUID(),
        material_id: crypto.randomUUID(),
        status: 'pending',
        gcs_path: `gs://tvtutor-raw-uploads/mock/${courseId}/${filename}`,
      },
      { status: 202 },
    )
  }

  // Forward to ingestion service when available.
  const tokenCookie = req.cookies.get('tl_token')?.value
  const authHeader =
    req.headers.get('authorization') ?? (tokenCookie ? `Bearer ${tokenCookie}` : null)

  if (!authHeader) {
    return NextResponse.json({ detail: 'unauthorized' }, { status: 401 })
  }

  const url = new URL(`${INGESTION_SERVICE_URL}/v1/ingest/upload`)
  url.searchParams.set('course_id', courseId)

  const formData = await req.formData()

  let upstream: Response
  try {
    upstream = await fetch(url.toString(), {
      method: 'POST',
      headers: { Authorization: authHeader },
      body: formData,
    })
  } catch {
    return NextResponse.json({ detail: 'ingestion service unavailable' }, { status: 502 })
  }

  const data: unknown = await upstream.json().catch(() => ({ detail: upstream.statusText }))
  return NextResponse.json(data, { status: upstream.status })
}
