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

// POST /api/assessment  →  POST /gaming/assessment/start
export async function POST(req: NextRequest) {
  const authHeader = getAuthHeader(req)
  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/assessment/start`, {
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
