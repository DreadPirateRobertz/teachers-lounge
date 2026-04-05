/**
 * Tests for lib/csp.ts — verifies that buildCsp produces correct directives
 * with and without a nonce value.
 */

import { buildCsp } from './csp'

describe('buildCsp', () => {
  it('includes default-src self and frame-ancestors none', () => {
    const csp = buildCsp()
    expect(csp).toContain("default-src 'self'")
    expect(csp).toContain("frame-ancestors 'none'")
    expect(csp).not.toContain('upgrade-insecure-requests')
    expect(csp).toContain("form-action 'self'")
  })

  it('without nonce: script-src is self only, no unsafe-inline', () => {
    const csp = buildCsp()
    expect(csp).toContain("script-src 'self'")
    const scriptSrc = csp.match(/script-src([^;]+)/)?.[1] ?? ''
    expect(scriptSrc).not.toContain('unsafe-inline')
    expect(scriptSrc).not.toContain('nonce-')
  })

  it('with nonce: script-src contains the nonce directive', () => {
    const nonce = 'abc123=='
    const csp = buildCsp(nonce)
    expect(csp).toContain(`'nonce-${nonce}'`)
    expect(csp).toContain("script-src 'self' 'nonce-abc123=='")
    const scriptSrc = csp.match(/script-src([^;]+)/)?.[1] ?? ''
    expect(scriptSrc).not.toContain('unsafe-inline')
  })

  it('with nonce: nonce value is embedded verbatim', () => {
    const nonce = 'dGVzdC1ub25jZQ=='
    const csp = buildCsp(nonce)
    expect(csp).toContain(`'nonce-${nonce}'`)
  })

  it('preserves style-src unsafe-inline for CSS-in-JS', () => {
    const csp = buildCsp()
    expect(csp).toContain("style-src 'self' 'unsafe-inline'")
  })

  it('includes worker-src blob for Three.js workers', () => {
    const csp = buildCsp()
    expect(csp).toContain('worker-src blob:')
  })

  it('returns a semicolon-separated string', () => {
    const csp = buildCsp()
    expect(csp).toMatch(/; /)
  })
})
