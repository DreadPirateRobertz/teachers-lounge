/**
 * API proxy for PATCH /api/user/onboarding.
 *
 * Forwards the request to the user-service `PATCH /users/{userId}/onboarding`
 * endpoint to mark the first-run wizard as complete.  The user ID is extracted
 * from the `tl_token` JWT cookie rather than a URL segment so the client does
 * not need to know its own ID at call-site.
 *
 * Returns 204 on success; 400 if the token is missing or the ID cannot be
 * parsed; upstream errors are forwarded verbatim.
 */

import { NextRequest, NextResponse } from 'next/server'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'

/**
 * Decode the `sub` claim from a JWT without verifying the signature.
 *
 * Safe here because the user-service re-validates the token on the upstream
 * call.  We only need the ID to construct the correct upstream URL.
 *
 * @param token - Raw JWT string.
 * @returns The `sub` claim string, or `null` on any parse error.
 */
function extractSub(token: string): string | null {
  try {
    const payload = token.split('.')[1]
    const decoded = JSON.parse(Buffer.from(payload, 'base64url').toString())
    return typeof decoded.sub === 'string' ? decoded.sub : null
  } catch {
    return null
  }
}

/**
 * PATCH /api/user/onboarding
 *
 * Marks the authenticated user's onboarding wizard as complete by proxying
 * `PATCH /users/{id}/onboarding` to the user-service.  Idempotent.
 *
 * @param req - Incoming Next.js request carrying a `tl_token` cookie.
 * @returns 204 No Content on success; 400 if token is absent/invalid.
 */
export async function PATCH(req: NextRequest) {
  const token = req.cookies.get('tl_token')?.value
  if (!token) {
    return NextResponse.json({ error: 'not authenticated' }, { status: 400 })
  }

  const userId = extractSub(token)
  if (!userId) {
    return NextResponse.json({ error: 'invalid token' }, { status: 400 })
  }

  const upstream = await fetch(`${USER_SERVICE_URL}/users/${userId}/onboarding`, {
    method: 'PATCH',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  if (!upstream.ok) {
    const contentType = upstream.headers.get('content-type') || ''
    const body = contentType.includes('application/json')
      ? await upstream.json()
      : await upstream.text()
    return NextResponse.json(body, { status: upstream.status })
  }

  return new NextResponse(null, { status: 204 })
}
