import type { NextConfig } from 'next'
import { buildCsp } from './lib/csp'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'
const TUTORING_SERVICE_URL = process.env.TUTORING_SERVICE_URL || 'http://tutoring-service:8000'

/**
 * Static CSP fallback for routes that bypass middleware (API routes,
 * static assets).  No nonce is included — script-src is 'self' only.
 *
 * Page routes receive a per-request nonce injected by middleware.ts
 * (tl-ixk), which overrides this header with `'nonce-<value>'` in
 * script-src, enabling Next.js hydration scripts without unsafe-inline.
 *
 * Style rationale:
 * - style-src 'self' 'unsafe-inline': Tailwind + CSS-in-JS require inline styles
 * - img-src data: blob:: canvas/Three.js screenshots + molecule renders
 * - media-src blob:: TTS audio via Web Audio API
 * - worker-src blob:: Three.js OffscreenCanvas workers
 */
const CSP = buildCsp()

const SECURITY_HEADERS = [
  { key: 'Content-Security-Policy', value: CSP },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
  { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
  // HSTS only in production — local dev runs plain HTTP
  ...(process.env.DISABLE_HSTS === 'true'
    ? []
    : [{ key: 'Strict-Transport-Security', value: 'max-age=63072000; includeSubDomains; preload' }]),
]

const config: NextConfig = {
  output: 'standalone',

  async headers() {
    return [
      {
        // Apply security headers to every route.
        source: '/(.*)',
        headers: SECURITY_HEADERS,
      },
    ]
  },

  // GKE internal DNS rewrites — forward backend traffic to services.
  // Note: auth endpoints go through /app/api/user/auth/ (route handler) so we
  // can manage the tl_token cookie. Everything else can rewrite directly.
  async rewrites() {
    return [
      {
        source: '/api/user/users/:path*',
        destination: `${USER_SERVICE_URL}/users/:path*`,
      },
      {
        source: '/api/user/webhooks/:path*',
        destination: `${USER_SERVICE_URL}/webhooks/:path*`,
      },
      {
        source: '/api/tutor/:path*',
        destination: `${TUTORING_SERVICE_URL}/:path*`,
      },
    ]
  },
}

export default config
