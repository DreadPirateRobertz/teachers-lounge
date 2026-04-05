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

/** GET /api/flashcards/due → GET /gaming/flashcards/due */
export async function GET(req: NextRequest) {
  const authHeader = getAuthHeader(req)
  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/flashcards/due`, {
    headers: { ...(authHeader ? { Authorization: authHeader } : {}) },
  })
  const data = await upstream.json()
  return NextResponse.json(data, { status: upstream.status })
}
