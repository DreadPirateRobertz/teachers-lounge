import { NextRequest, NextResponse } from 'next/server'

const GAMING_SERVICE_URL = process.env.GAMING_SERVICE_URL || 'http://gaming-service:8083'

/** Reads the auth token from the Authorization header or tl_token cookie. */
function getAuthHeader(req: NextRequest): string | undefined {
  return (
    req.headers.get('authorization') ??
    (req.cookies.get('tl_token')?.value
      ? `Bearer ${req.cookies.get('tl_token')!.value}`
      : undefined)
  )
}

/**
 * GET /api/flashcards/export/anki → GET /gaming/flashcards/export/anki
 *
 * Forwards the binary .apkg response as-is, preserving Content-Disposition
 * so the browser treats it as a file download.
 */
export async function GET(req: NextRequest) {
  const authHeader = getAuthHeader(req)
  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/flashcards/export/anki`, {
    headers: { ...(authHeader ? { Authorization: authHeader } : {}) },
  })

  const blob = await upstream.arrayBuffer()
  const contentDisposition =
    upstream.headers.get('content-disposition') ?? 'attachment; filename="flashcards.apkg"'
  const contentType = upstream.headers.get('content-type') ?? 'application/octet-stream'

  return new NextResponse(blob, {
    status: upstream.status,
    headers: {
      'Content-Type': contentType,
      'Content-Disposition': contentDisposition,
    },
  })
}
