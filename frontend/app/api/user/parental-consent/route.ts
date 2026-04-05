/**
 * Parental consent request endpoint — skeleton.
 *
 * Proxies a consent request to the user-service.  The user-service is
 * responsible for recording the pending consent state and triggering a
 * consent email to the guardian.
 *
 * TODO (tl-5zx): wire guardian email delivery in user-service once
 * SendGrid/SES is configured for transactional consent emails.
 *
 * Request body:
 *   { user_id: string, guardian_email: string }
 *
 * Success response (202):
 *   { message: "consent email sent" }
 *
 * Error responses:
 *   400 — missing or invalid fields
 *   500 — upstream user-service error
 */
import { NextRequest, NextResponse } from 'next/server'

const USER_SERVICE = process.env.USER_SERVICE_URL ?? 'http://localhost:8080'

/**
 * POST /api/user/parental-consent
 *
 * Forwards a parental consent request to the user-service.
 *
 * @param request - The incoming Next.js request.
 * @returns JSON response with status message or error.
 */
export async function POST(request: NextRequest): Promise<NextResponse> {
  let body: { user_id?: string; guardian_email?: string }
  try {
    body = await request.json()
  } catch {
    return NextResponse.json({ error: 'Invalid JSON body' }, { status: 400 })
  }

  if (!body.user_id || !body.guardian_email) {
    return NextResponse.json({ error: 'user_id and guardian_email are required' }, { status: 400 })
  }

  // Validate user_id is a UUID to prevent path traversal / SSRF
  const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
  if (!UUID_RE.test(body.user_id)) {
    return NextResponse.json({ error: 'Invalid user_id' }, { status: 400 })
  }

  try {
    const upstream = await fetch(`${USER_SERVICE}/users/${body.user_id}/parental-consent`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ guardian_email: body.guardian_email }),
    })

    const data = await upstream.json()

    if (!upstream.ok) {
      return NextResponse.json(
        { error: data.error ?? 'Failed to request consent' },
        { status: upstream.status },
      )
    }

    return NextResponse.json({ message: 'consent email sent' }, { status: 202 })
  } catch {
    return NextResponse.json({ error: 'User service unavailable' }, { status: 500 })
  }
}
