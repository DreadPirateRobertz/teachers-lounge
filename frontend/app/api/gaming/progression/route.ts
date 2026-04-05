import { NextRequest, NextResponse } from 'next/server'

const GAMING_SERVICE_URL = process.env.GAMING_SERVICE_URL

/**
 * Extracts the Authorization header from cookie or request header.
 *
 * @param req - The incoming Next.js request.
 * @returns A headers record with Authorization set, or empty if unauthenticated.
 */
function authHeader(req: NextRequest): Record<string, string> {
  const bearer =
    req.headers.get('authorization') ||
    (req.cookies.get('tl_token')?.value ? `Bearer ${req.cookies.get('tl_token')!.value}` : null)
  return bearer ? { Authorization: bearer } : {}
}

/**
 * GET /api/gaming/progression — Returns the boss progression trail for the
 * authenticated user. Each node reports whether the boss is defeated, currently
 * available, or locked behind undefeated prerequisites.
 *
 * Proxies to gaming-service GET /gaming/boss/progression.
 * Falls back to a mock trail when GAMING_SERVICE_URL is not configured.
 */
export async function GET(req: NextRequest) {
  if (!GAMING_SERVICE_URL) {
    return NextResponse.json(mockProgression())
  }

  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/boss/progression`, {
    headers: { 'Content-Type': 'application/json', ...authHeader(req) },
    cache: 'no-store',
  })

  if (!upstream.ok) {
    return NextResponse.json(
      { error: 'Failed to fetch boss progression' },
      { status: upstream.status },
    )
  }

  return NextResponse.json(await upstream.json())
}

// ── Mock data (used when GAMING_SERVICE_URL is not set) ──────────────────────

/** Mock boss progression trail for local development. */
function mockProgression() {
  return {
    total_defeated: 1,
    nodes: [
      {
        boss_id: 'the_atom',
        name: 'THE ATOM',
        topic: 'Atomic Structure',
        tier: 1,
        victory_xp: 500,
        primary_color: '#00aaff',
        state: 'defeated',
      },
      {
        boss_id: 'the_bonder',
        name: 'THE BONDER',
        topic: 'Chemical Bonding',
        tier: 2,
        victory_xp: 750,
        primary_color: '#00ff88',
        state: 'current',
      },
      {
        boss_id: 'name_lord',
        name: 'NAME LORD',
        topic: 'Nomenclature',
        tier: 3,
        victory_xp: 1000,
        primary_color: '#ff00aa',
        state: 'locked',
      },
      {
        boss_id: 'the_stereochemist',
        name: 'THE STEREOCHEMIST',
        topic: 'Stereochemistry',
        tier: 4,
        victory_xp: 1250,
        primary_color: '#cc44ff',
        state: 'locked',
      },
      {
        boss_id: 'the_reactor',
        name: 'THE REACTOR',
        topic: 'Reaction Mechanisms',
        tier: 5,
        victory_xp: 1500,
        primary_color: '#ff6600',
        state: 'locked',
      },
      {
        boss_id: 'final_boss',
        name: 'FINAL BOSS',
        topic: 'Advanced Organic Chemistry',
        tier: 6,
        victory_xp: 3000,
        primary_color: '#ff0055',
        state: 'locked',
      },
    ],
  }
}
