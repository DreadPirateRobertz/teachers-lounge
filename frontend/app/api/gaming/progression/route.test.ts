/**
 * @jest-environment node
 * @fileoverview Tests for the /api/gaming/progression Next.js route handler.
 *
 * Verifies mock-mode behavior (no GAMING_SERVICE_URL). Upstream proxying
 * is covered by integration tests.
 */

import { NextRequest } from 'next/server'
import { GET } from './route'

beforeEach(() => {
  delete process.env.GAMING_SERVICE_URL
})

describe('GET /api/gaming/progression', () => {
  it('returns 200 in mock mode', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    expect(resp.status).toBe(200)
  })

  it('response contains nodes array', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    expect(Array.isArray(data.nodes)).toBe(true)
    expect(data.nodes.length).toBeGreaterThan(0)
  })

  it('response contains total_defeated number', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    expect(typeof data.total_defeated).toBe('number')
  })

  it('every node has required fields', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    for (const node of data.nodes) {
      expect(node.boss_id).toBeTruthy()
      expect(node.name).toBeTruthy()
      expect(node.tier).toBeGreaterThan(0)
      expect(['defeated', 'current', 'locked']).toContain(node.state)
      expect(node.primary_color).toMatch(/^#[0-9a-f]{6}$/i)
    }
  })

  it('nodes are ordered by ascending tier', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    for (let i = 1; i < data.nodes.length; i++) {
      expect(data.nodes[i].tier).toBeGreaterThanOrEqual(data.nodes[i - 1].tier)
    }
  })

  it('mock has exactly one current node and rest are defeated or locked', async () => {
    const req = new NextRequest('http://localhost/api/gaming/progression')
    const resp = await GET(req)
    const data = await resp.json()
    const currentNodes = data.nodes.filter((n: { state: string }) => n.state === 'current')
    expect(currentNodes).toHaveLength(1)
  })
})
