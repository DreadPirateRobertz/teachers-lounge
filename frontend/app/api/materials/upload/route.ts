import { NextRequest, NextResponse } from 'next/server'

const INGESTION_SERVICE_URL = process.env.INGESTION_SERVICE_URL

export async function POST(req: NextRequest): Promise<NextResponse> {
  const courseId = req.nextUrl.searchParams.get('course_id')
  if (!courseId) {
    return NextResponse.json({ detail: 'course_id is required' }, { status: 400 })
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
    req.headers.get('authorization') ??
    (tokenCookie ? `Bearer ${tokenCookie}` : null)

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
