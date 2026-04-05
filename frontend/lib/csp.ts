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
 * `'nonce-<value>'` in place of `'unsafe-inline'`, satisfying
 * Level-2 CSP without weakening XSS protection.
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
  const scriptSrc = nonce ? `'self' 'nonce-${nonce}'` : "'self'"

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

  // Only upgrade to HTTPS in production — local dev runs over plain HTTP
  if (process.env.NODE_ENV === 'production' && process.env.DISABLE_HSTS !== 'true') {
    directives.push('upgrade-insecure-requests')
  }

  return directives.join('; ')
}
