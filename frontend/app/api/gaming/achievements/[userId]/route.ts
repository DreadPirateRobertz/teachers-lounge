import { NextRequest, NextResponse } from 'next/server'

const GAMING_SERVICE_URL = process.env.GAMING_SERVICE_URL || 'http://gaming-service:8083'

/** GET /api/gaming/achievements/[userId] — proxy to gaming-service */
export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ userId: string }> },
) {
  const token = req.cookies.get('tl_token')?.value
  if (!token) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
  }

  const { userId } = await params
  const upstream = await fetch(
    `${GAMING_SERVICE_URL}/gaming/achievements/${encodeURIComponent(userId)}`,
    { headers: { Authorization: `Bearer ${token}` } },
  )

  const body = await upstream.json()
  return NextResponse.json(body, { status: upstream.status })
}
