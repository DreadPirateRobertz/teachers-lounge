/**
 * Tests for POST /api/user/parental-consent — parental consent request proxy.
 */
import { NextRequest } from 'next/server'
import { POST } from './route'

function makeRequest(body: unknown): NextRequest {
  return new NextRequest('http://localhost/api/user/parental-consent', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

describe('POST /api/user/parental-consent', () => {
  beforeEach(() => {
    jest.resetAllMocks()
  })

  it('returns 400 when body is missing user_id', async () => {
    const res = await POST(makeRequest({ guardian_email: 'parent@example.com' }))
    expect(res.status).toBe(400)
    const body = await res.json()
    expect(body.error).toMatch(/user_id/)
  })

  it('returns 400 when body is missing guardian_email', async () => {
    const res = await POST(makeRequest({ user_id: 'uid-123' }))
    expect(res.status).toBe(400)
    const body = await res.json()
    expect(body.error).toMatch(/guardian_email/)
  })

  it('returns 400 when body is not valid JSON', async () => {
    const req = new NextRequest('http://localhost/api/user/parental-consent', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: 'not-json',
    })
    const res = await POST(req)
    expect(res.status).toBe(400)
  })

  it('proxies to user-service and returns 202 on success', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      status: 202,
      json: async () => ({ message: 'ok' }),
    }) as jest.Mock

    const res = await POST(makeRequest({ user_id: 'uid-123', guardian_email: 'p@ex.com' }))
    expect(res.status).toBe(202)
    const body = await res.json()
    expect(body.message).toBe('consent email sent')
  })

  it('forwards upstream error status when user-service rejects', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 422,
      json: async () => ({ error: 'invalid guardian email' }),
    }) as jest.Mock

    const res = await POST(makeRequest({ user_id: 'uid-123', guardian_email: 'p@ex.com' }))
    expect(res.status).toBe(422)
    const body = await res.json()
    expect(body.error).toBe('invalid guardian email')
  })

  it('returns 500 when user-service is unreachable', async () => {
    global.fetch = jest.fn().mockRejectedValue(new Error('ECONNREFUSED')) as jest.Mock

    const res = await POST(makeRequest({ user_id: 'uid-123', guardian_email: 'p@ex.com' }))
    expect(res.status).toBe(500)
  })
})
