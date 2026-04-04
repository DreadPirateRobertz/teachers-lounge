import { NextRequest, NextResponse } from 'next/server'

// Paths that don't require authentication
const PUBLIC_PATHS = ['/login', '/register', '/subscribe']

// Post-auth paths under /subscribe (success/cancel after Stripe redirect)
const SUBSCRIBE_POST_AUTH = ['/subscribe/success', '/subscribe/cancel']

function parseJwtPayload(token: string): Record<string, unknown> {
  try {
    const payload = token.split('.')[1]
    return JSON.parse(Buffer.from(payload, 'base64url').toString())
  } catch {
    return {}
  }
}

export function middleware(req: NextRequest) {
  const token = req.cookies.get('tl_token')?.value
  const path = req.nextUrl.pathname

  const isPostAuth = SUBSCRIBE_POST_AUTH.some(p => path.startsWith(p))
  const isPublic = !isPostAuth && PUBLIC_PATHS.some(p => path.startsWith(p))

  if (!token && !isPublic) {
    const loginUrl = new URL('/login', req.url)
    loginUrl.searchParams.set('next', path)
    return NextResponse.redirect(loginUrl)
  }

  if (token && isPublic) {
    return NextResponse.redirect(new URL('/', req.url))
  }

  // Guard /dashboard: redirect to /subscribe if no active subscription
  if (token && path.startsWith('/dashboard')) {
    const claims = parseJwtPayload(token)
    const subStatus = claims.sub_status as string | undefined
    if (subStatus !== 'active' && subStatus !== 'trialing') {
      return NextResponse.redirect(new URL('/subscribe', req.url))
    }
  }
}

export const config = {
  // Skip API routes, static files, and Next.js internals
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico).*)'],
}
