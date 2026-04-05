import { NextRequest, NextResponse } from 'next/server'

const GAMING_SERVICE_URL = process.env.GAMING_SERVICE_URL || 'http://gaming-service:8083'

function getAuthHeader(req: NextRequest): string | undefined {
  return (
    req.headers.get('authorization') ??
    (req.cookies.get('tl_token')?.value
      ? `Bearer ${req.cookies.get('tl_token')!.value}`
      : undefined)
  )
}

type Params = { params: Promise<{ sessionId: string }> }

// GET /api/assessment/sessions/{id}  →  GET /gaming/assessment/sessions/{id}
export async function GET(req: NextRequest, { params }: Params) {
  const { sessionId } = await params
  const authHeader = getAuthHeader(req)
  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/assessment/sessions/${sessionId}`, {
    headers: { ...(authHeader ? { Authorization: authHeader } : {}) },
  })
  const data = await upstream.json()
  return NextResponse.json(data, { status: upstream.status })
}

// POST /api/assessment/sessions/{id}  →  POST /gaming/assessment/sessions/{id}/answer
export async function POST(req: NextRequest, { params }: Params) {
  const { sessionId } = await params
  const authHeader = getAuthHeader(req)
  const upstream = await fetch(
    `${GAMING_SERVICE_URL}/gaming/assessment/sessions/${sessionId}/answer`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(authHeader ? { Authorization: authHeader } : {}),
      },
      body: await req.text(),
    },
  )
  const data = await upstream.json()
  return NextResponse.json(data, { status: upstream.status })
}
