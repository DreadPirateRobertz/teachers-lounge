import { NextRequest, NextResponse } from 'next/server'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'

const ALLOWED_PLAN_IDS = new Set(['monthly', 'quarterly', 'semesterly'])

function parseJwtPayload(token: string): Record<string, unknown> {
  try {
    const payload = token.split('.')[1]
    return JSON.parse(Buffer.from(payload, 'base64url').toString())
  } catch {
    return {}
  }
}

export async function POST(req: NextRequest) {
  const token = req.cookies.get('tl_token')?.value
  if (!token) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
  }

  const claims = parseJwtPayload(token)
  const userId = claims.sub as string | undefined
  if (!userId) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
  }

  const { planId } = await req.json()
  if (!planId || !ALLOWED_PLAN_IDS.has(planId)) {
    return NextResponse.json({ error: 'Invalid plan' }, { status: 400 })
  }

  const upstream = await fetch(`${USER_SERVICE_URL}/users/${userId}/subscription`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ plan_id: planId }),
  })

  if (!upstream.ok) {
    const text = await upstream.text()
    return NextResponse.json({ error: text || 'Upstream error' }, { status: upstream.status })
  }

  const { checkout_url } = await upstream.json()
  return NextResponse.json({ checkoutUrl: checkout_url })
}
