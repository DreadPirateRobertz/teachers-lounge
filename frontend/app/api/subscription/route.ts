import { NextRequest, NextResponse } from 'next/server'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'

function parseJwtPayload(token: string): Record<string, unknown> {
  try {
    const payload = token.split('.')[1]
    return JSON.parse(Buffer.from(payload, 'base64url').toString())
  } catch {
    return {}
  }
}

export async function GET(req: NextRequest) {
  const token = req.cookies.get('tl_token')?.value
  if (!token) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
  }

  const claims = parseJwtPayload(token)
  const userId = claims.sub as string | undefined
  if (!userId) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
  }

  const upstream = await fetch(`${USER_SERVICE_URL}/users/${userId}/subscription`, {
    headers: { 'Authorization': `Bearer ${token}` },
  })

  const body = await upstream.json()
  return NextResponse.json(body, { status: upstream.status })
}
