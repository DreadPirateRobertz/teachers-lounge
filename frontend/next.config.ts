import type { NextConfig } from 'next'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'
const TUTORING_SERVICE_URL = process.env.TUTORING_SERVICE_URL || 'http://tutoring-service:8000'

/**
 * Content-Security-Policy header value.
 *
 * Policy rationale:
 * - default-src 'self': block all unlisted origins by default
 * - script-src 'self' 'unsafe-inline': Next.js inline scripts required for hydration;
 *   nonce-based CSP is preferred long-term but requires middleware refactor
 * - style-src 'self' 'unsafe-inline': Tailwind + emotion CSS-in-JS require inline styles
 * - img-src 'self' data: blob:: data: for canvas/Three.js screenshots, blob: for
 *   generated molecule renders
 * - font-src 'self': web fonts served from /public
 * - connect-src 'self': fetch/XHR only to same origin (API routes proxy to backends)
 * - media-src 'self' blob:: blob: for TTS audio player (Web Audio API)
 * - worker-src blob:: Three.js OffscreenCanvas workers
 * - frame-ancestors 'none': blocks clickjacking
 * - form-action 'self': prevents form POST hijacking
 * - upgrade-insecure-requests: force HTTPS in prod
 */
const CSP = [
  "default-src 'self'",
  "script-src 'self' 'unsafe-inline'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self'",
  "connect-src 'self'",
  "media-src 'self' blob:",
  "worker-src blob:",
  "frame-ancestors 'none'",
  "form-action 'self'",
  "upgrade-insecure-requests",
].join('; ')

const SECURITY_HEADERS = [
  { key: 'Content-Security-Policy', value: CSP },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
  { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
  { key: 'Strict-Transport-Security', value: 'max-age=63072000; includeSubDomains; preload' },
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
