/**
 * @jest-environment node
 * @fileoverview Tests for the /api/gaming/shop Next.js route handlers.
 *
 * Verifies that GET returns the mock catalog and POST returns a purchase
 * response when GAMING_SERVICE_URL is not set (local dev / mock mode).
 * Upstream proxying is covered by integration tests.
 */

import { NextRequest } from 'next/server'
import { GET, POST } from './route'

beforeEach(() => {
  delete process.env.GAMING_SERVICE_URL
})

// ── GET /api/gaming/shop ──────────────────────────────────────────────────────

describe('GET /api/gaming/shop', () => {
  it('returns 200 with mock catalog when GAMING_SERVICE_URL is unset', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await GET(req)
    expect(resp.status).toBe(200)
  })

  it('catalog contains exactly 4 items', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await GET(req)
    const data = await resp.json()
    expect(data.items).toHaveLength(4)
  })

  it('every catalog item has a positive gem_cost', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await GET(req)
    const data = await resp.json()
    for (const item of data.items) {
      expect(item.gem_cost).toBeGreaterThan(0)
    }
  })

  it('every catalog item has type, label, icon, and description', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop')
    const resp = await GET(req)
    const data = await resp.json()
    for (const item of data.items) {
      expect(item.type).toBeTruthy()
      expect(item.label).toBeTruthy()
      expect(item.icon).toBeTruthy()
      expect(item.description).toBeTruthy()
    }
  })
})

// ── POST /api/gaming/shop ─────────────────────────────────────────────────────

describe('POST /api/gaming/shop', () => {
  it('returns 200 with purchase response shape for mock mode', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop', {
      method: 'POST',
      body: JSON.stringify({ user_id: 'u1', power_up: 'shield' }),
      headers: { 'Content-Type': 'application/json' },
    })
    const resp = await POST(req)
    expect(resp.status).toBe(200)
    const data = await resp.json()
    expect(data.power_up).toBe('shield')
    expect(typeof data.gems_left).toBe('number')
    expect(typeof data.new_count).toBe('number')
  })

  it('echoes back the requested power_up type', async () => {
    const req = new NextRequest('http://localhost/api/gaming/shop', {
      method: 'POST',
      body: JSON.stringify({ user_id: 'u1', power_up: 'critical' }),
      headers: { 'Content-Type': 'application/json' },
    })
    const resp = await POST(req)
    const data = await resp.json()
    expect(data.power_up).toBe('critical')
  })
})
