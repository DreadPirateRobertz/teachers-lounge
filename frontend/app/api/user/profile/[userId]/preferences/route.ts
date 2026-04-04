import { NextRequest, NextResponse } from 'next/server'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'

type Params = { params: Promise<{ userId: string }> }

function getAuthHeader(req: NextRequest): string | undefined {
  return (
    req.headers.get('authorization') ??
    (req.cookies.get('tl_token')?.value
      ? `Bearer ${req.cookies.get('tl_token')!.value}`
      : undefined)
  )
}

// PATCH /api/user/profile/{userId}/preferences
//   → PATCH /users/{userId}/preferences  (user-service)
export async function PATCH(req: NextRequest, { params }: Params) {
  const { userId } = await params
  const authHeader = getAuthHeader(req)

  const upstream = await fetch(`${USER_SERVICE_URL}/users/${userId}/preferences`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      ...(authHeader ? { Authorization: authHeader } : {}),
    },
    body: await req.text(),
  })

  const contentType = upstream.headers.get('content-type') || ''
  const body = contentType.includes('application/json')
    ? await upstream.json()
    : await upstream.text()

  return NextResponse.json(body, { status: upstream.status })
}
