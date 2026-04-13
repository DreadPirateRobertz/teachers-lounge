/**
 * GET /api/user/profile/[userId]
 *
 * Proxies to the user-service GET /users/{userId}/profile endpoint,
 * forwarding the caller's auth token from the Authorization header or the
 * `tl_token` cookie.
 */
import { NextRequest, NextResponse } from 'next/server'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'

type Params = { params: Promise<{ userId: string }> }

/** Reads the auth token from the Authorization header or tl_token cookie. */
function getAuthHeader(req: NextRequest): string | undefined {
  return (
    req.headers.get('authorization') ??
    (req.cookies.get('tl_token')?.value
      ? `Bearer ${req.cookies.get('tl_token')!.value}`
      : undefined)
  )
}

/** GET /api/user/profile/{userId} → GET /users/{userId}/profile (user-service) */
export async function GET(req: NextRequest, { params }: Params) {
  const { userId } = await params
  const authHeader = getAuthHeader(req)

  const upstream = await fetch(`${USER_SERVICE_URL}/users/${userId}/profile`, {
    headers: {
      'Content-Type': 'application/json',
      ...(authHeader ? { Authorization: authHeader } : {}),
    },
  })

  const contentType = upstream.headers.get('content-type') || ''
  const body = contentType.includes('application/json')
    ? await upstream.json()
    : await upstream.text()

  return NextResponse.json(body, { status: upstream.status })
}
