/**
 * Tests for next.config.ts — verifies that security headers are correctly
 * defined and include all required directives.
 */

describe('next.config security headers', () => {
  let headers: { key: string; value: string }[]

  beforeAll(async () => {
    jest.resetModules()
    const mod = await import('./next.config')
    const rules = await mod.default.headers!()
    headers = rules[0].headers
  })

  it('applies headers to all routes via /(.*) matcher', async () => {
    jest.resetModules()
    const mod = await import('./next.config')
    const rules = await mod.default.headers!()
    expect(rules).toHaveLength(1)
    expect(rules[0].source).toBe('/(.*)')
  })

  it('includes Content-Security-Policy with frame-ancestors and upgrade directives', () => {
    const csp = headers.find((h) => h.key === 'Content-Security-Policy')
    expect(csp).toBeDefined()
    expect(csp!.value).toContain("default-src 'self'")
    expect(csp!.value).toContain("frame-ancestors 'none'")
    expect(csp!.value).not.toContain('upgrade-insecure-requests')
    expect(csp!.value).toContain("form-action 'self'")
  })

  it('sets X-Frame-Options to DENY', () => {
    const h = headers.find((hdr) => hdr.key === 'X-Frame-Options')
    expect(h?.value).toBe('DENY')
  })

  it('sets X-Content-Type-Options to nosniff', () => {
    const h = headers.find((hdr) => hdr.key === 'X-Content-Type-Options')
    expect(h?.value).toBe('nosniff')
  })

  it('does not set HSTS — handled by reverse proxy in production', () => {
    const h = headers.find((hdr) => hdr.key === 'Strict-Transport-Security')
    expect(h).toBeUndefined()
  })

  it('sets Referrer-Policy', () => {
    const h = headers.find((hdr) => hdr.key === 'Referrer-Policy')
    expect(h).toBeDefined()
  })

  it('sets Permissions-Policy disabling camera, microphone, geolocation', () => {
    const h = headers.find((hdr) => hdr.key === 'Permissions-Policy')
    expect(h?.value).toContain('camera=()')
    expect(h?.value).toContain('microphone=()')
    expect(h?.value).toContain('geolocation=()')
  })

  it('does not contain unsafe-inline in script-src (nonce migration tl-ixk)', () => {
    const csp = headers.find((h) => h.key === 'Content-Security-Policy')
    expect(csp).toBeDefined()
    const scriptSrcMatch = csp!.value.match(/script-src([^;]+)/)
    expect(scriptSrcMatch).not.toBeNull()
    expect(scriptSrcMatch![1]).not.toContain('unsafe-inline')
  })
})
