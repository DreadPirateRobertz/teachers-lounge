import { NextRequest, NextResponse } from 'next/server'

const ANALYTICS_SERVICE_URL = process.env.ANALYTICS_SERVICE_URL || 'http://analytics-service:8085'

export async function GET(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const token = req.cookies.get('tl_token')?.value
  if (!token) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
  }

  const { path } = await params
  const upstreamPath = path.join('/')
  const search = req.nextUrl.search

  const upstream = await fetch(`${ANALYTICS_SERVICE_URL}/v1/analytics/${upstreamPath}${search}`, {
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  })

  const body = await upstream.json()
  return NextResponse.json(body, { status: upstream.status })
}
