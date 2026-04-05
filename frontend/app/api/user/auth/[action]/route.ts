import { NextRequest, NextResponse } from 'next/server'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'

const ALLOWED_ACTIONS = new Set(['login', 'register', 'logout', 'refresh'])

// Proxy auth requests to User Service, manage tl_token cookie
export async function POST(req: NextRequest, { params }: { params: Promise<{ action: string }> }) {
  const { action } = await params

  if (!ALLOWED_ACTIONS.has(action)) {
    return NextResponse.json({ error: 'Not found' }, { status: 404 })
  }

  const upstream = await fetch(`${USER_SERVICE_URL}/auth/${action}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Real-IP': req.headers.get('x-forwarded-for') || '127.0.0.1',
      // Forward refresh token cookie to User Service for refresh/logout
      Cookie: req.headers.get('cookie') || '',
    },
    body: action === 'logout' ? undefined : await req.text(),
  })

  const contentType = upstream.headers.get('content-type') || ''
  const body = contentType.includes('application/json')
    ? await upstream.json()
    : await upstream.text()

  const res = NextResponse.json(body, { status: upstream.status })

  // Forward httpOnly refresh_token cookie from User Service
  upstream.headers.forEach((value, key) => {
    if (key.toLowerCase() === 'set-cookie') {
      res.headers.append('Set-Cookie', value)
    }
  })

  // Set JS-readable access token cookie for middleware auth check
  if (upstream.ok && typeof body === 'object' && body.access_token) {
    res.cookies.set('tl_token', body.access_token, {
      httpOnly: false,
      sameSite: 'lax',
      maxAge: 60 * 15, // 15 min — matches User Service access token TTL
      path: '/',
    })
  }

  // Clear token on logout
  if (action === 'logout') {
    res.cookies.delete('tl_token')
  }

  return res
}
