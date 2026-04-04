import { NextRequest, NextResponse } from 'next/server'

const GAMING_SERVICE_URL = process.env.GAMING_SERVICE_URL || 'http://gaming-service:8083'

export async function GET(req: NextRequest) {
  const token = req.cookies.get('tl_token')?.value
  if (!token) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
  }

  const period = req.nextUrl.searchParams.get('period') || 'all_time'
  const upstream = await fetch(
    `${GAMING_SERVICE_URL}/gaming/leaderboard?period=${encodeURIComponent(period)}`,
    { headers: { Authorization: `Bearer ${token}` } },
  )

  const body = await upstream.json()
  return NextResponse.json(body, { status: upstream.status })
}
