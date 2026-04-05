/**
 * Tests for GET /api/health
 */

describe('GET /api/health', () => {
  it('returns 200 with service status', async () => {
    const { GET } = await import('./route')
    const res = await GET()
    const data = await res.json()

    expect(res.status).toBe(200)
    expect(data).toEqual({ status: 'ok', service: 'frontend' })
  })
})
