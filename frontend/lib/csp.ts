/**
 * Content-Security-Policy builder.
 *
 * Centralises CSP construction so that both next.config.ts (static header
 * fallback for API/asset routes) and middleware.ts (per-request nonce for
 * page routes) share the same directive set.
 */

/**
 * Build a Content-Security-Policy header value.
 *
 * When a `nonce` is supplied the `script-src` directive uses
 * `'nonce-<value>' 'strict-dynamic'` in place of `'unsafe-inline'`.
 * `'strict-dynamic'` allows Next.js to load code-split chunks that are
 * dynamically injected by nonce-trusted bootstrap scripts, satisfying
 * CSP Level 3 without `'unsafe-inline'` or a per-chunk URL allowlist.
 *
 * Without a nonce (e.g. static API routes that bypass middleware)
 * `script-src` falls back to `'self'` only — no inline scripts are
 * permitted on those routes.
 *
 * @param nonce - Base64-encoded random value generated per request.
 *   When omitted the resulting policy has no inline-script allowance.
 * @returns Semicolon-separated CSP header string ready for use in an
 *   HTTP `Content-Security-Policy` header.
 */
export function buildCsp(nonce?: string): string {
  const scriptSrc = nonce ? `'self' 'nonce-${nonce}' 'strict-dynamic'` : "'self'"

  const directives = [
    "default-src 'self'",
    `script-src ${scriptSrc}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "font-src 'self'",
    "connect-src 'self'",
    "media-src 'self' blob:",
    'worker-src blob:',
    "frame-ancestors 'none'",
    "form-action 'self'",
  ]

  // upgrade-insecure-requests is handled by the reverse proxy (nginx/caddy)
  // in production. Next.js standalone bakes CSP at build time, so we cannot
  // toggle it with runtime env vars.

  return directives.join('; ')
}
