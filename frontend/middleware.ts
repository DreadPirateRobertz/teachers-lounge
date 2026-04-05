import { NextRequest, NextResponse } from 'next/server'
import { buildCsp } from './lib/csp'

// Paths that don't require authentication
const PUBLIC_PATHS = ['/login', '/register', '/subscribe']

// Post-auth paths under /subscribe (success/cancel after Stripe redirect)
const SUBSCRIBE_POST_AUTH = ['/subscribe/success', '/subscribe/cancel']

/**
 * Generate a cryptographically random nonce for CSP.
 *
 * Uses the Web Crypto API available in the Next.js Edge Runtime.  The nonce
 * is 16 random bytes encoded as base64, giving 128 bits of entropy —
 * sufficient to make nonce prediction infeasible.
 *
 * @returns Base64-encoded random nonce string.
 */
export function generateNonce(): string {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return Buffer.from(bytes).toString('base64')
}

/**
 * Parse a JWT payload segment without verifying the signature.
 *
 * Used only for client-side routing decisions (subscription gate).  The
 * authoritative check happens in the backend service on every API call.
 *
 * @param token - Raw JWT string (`header.payload.signature`).
 * @returns Decoded payload object, or `{}` on any parse failure.
 */
function parseJwtPayload(token: string): Record<string, unknown> {
  try {
    const payload = token.split('.')[1]
    return JSON.parse(Buffer.from(payload, 'base64url').toString())
  } catch {
    return {}
  }
}

/**
 * Next.js Edge Middleware — authentication guard + CSP nonce injection.
 *
 * Runs on every matched page route (see `config.matcher`).  Responsibilities:
 * 1. Generate a per-request CSP nonce.
 * 2. Set `Content-Security-Policy` response header using the nonce so
 *    Next.js hydration inline scripts are allowed without `unsafe-inline`.
 * 3. Forward the nonce to Server Components via the `x-nonce` request header
 *    so layouts can pass `nonce` to `<Script>` elements.
 * 4. Enforce authentication — redirect unauthenticated users to `/login` and
 *    authenticated users away from public-only routes.
 * 5. Guard `/dashboard` behind an active Stripe subscription.
 *
 * @param req - Incoming Next.js edge request.
 * @returns A `NextResponse` (redirect, or `next()` with injected headers).
 */
export function middleware(req: NextRequest) {
  const nonce = generateNonce()
  const token = req.cookies.get('tl_token')?.value
  const path = req.nextUrl.pathname

  const isPostAuth = SUBSCRIBE_POST_AUTH.some((p) => path.startsWith(p))
  const isPublic = !isPostAuth && PUBLIC_PATHS.some((p) => path.startsWith(p))

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

  // Build the request with x-nonce so Server Components can read it.
  const requestHeaders = new Headers(req.headers)
  requestHeaders.set('x-nonce', nonce)

  const response = NextResponse.next({
    request: { headers: requestHeaders },
  })

  // Override CSP with the per-request nonce so Next.js hydration scripts pass
  // without relying on unsafe-inline.
  response.headers.set('Content-Security-Policy', buildCsp(nonce))

  return response
}

export const config = {
  // Skip API routes, static files, and Next.js internals
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico).*)'],
}
