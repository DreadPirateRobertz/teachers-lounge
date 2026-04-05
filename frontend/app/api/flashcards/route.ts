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

/** GET /api/flashcards → GET /gaming/flashcards */
export async function GET(req: NextRequest) {
  const authHeader = getAuthHeader(req)
  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/flashcards`, {
    headers: { ...(authHeader ? { Authorization: authHeader } : {}) },
  })
  const data = await upstream.json()
  return NextResponse.json(data, { status: upstream.status })
}

/** POST /api/flashcards → POST /gaming/flashcards/generate */
export async function POST(req: NextRequest) {
  const authHeader = getAuthHeader(req)
  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/flashcards/generate`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(authHeader ? { Authorization: authHeader } : {}),
    },
    body: await req.text(),
  })
  const data = await upstream.json()
  return NextResponse.json(data, { status: upstream.status })
}
