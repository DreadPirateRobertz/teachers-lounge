import { NextRequest, NextResponse } from 'next/server'

// Paths that don't require authentication
const PUBLIC_PATHS = ['/login', '/register', '/subscribe']

export function middleware(req: NextRequest) {
  const token = req.cookies.get('tl_token')?.value
  const path = req.nextUrl.pathname
  const isPublic = PUBLIC_PATHS.some(p => path.startsWith(p))

  if (!token && !isPublic) {
    const loginUrl = new URL('/login', req.url)
    loginUrl.searchParams.set('next', path)
    return NextResponse.redirect(loginUrl)
  }

  if (token && isPublic) {
    return NextResponse.redirect(new URL('/', req.url))
  }
}

export const config = {
  // Skip API routes, static files, and Next.js internals
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico).*)'],
}
