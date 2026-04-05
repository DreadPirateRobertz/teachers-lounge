import { NextRequest, NextResponse } from 'next/server'

const GAMING_SERVICE_URL = process.env.GAMING_SERVICE_URL

/**
 * Extracts the Authorization header from cookie or request header.
 *
 * @param req - The incoming Next.js request.
 * @returns A headers object with Authorization set, or empty if unauthenticated.
 */
function authHeader(req: NextRequest): Record<string, string> {
  const bearer =
    req.headers.get('authorization') ||
    (req.cookies.get('tl_token')?.value ? `Bearer ${req.cookies.get('tl_token')!.value}` : null)
  return bearer ? { Authorization: bearer } : {}
}

/**
 * GET /api/gaming/shop — Returns the full power-up shop catalog.
 *
 * Proxies to the gaming-service catalog endpoint. Falls back to a mock
 * catalog when GAMING_SERVICE_URL is not configured (local dev).
 */
export async function GET(req: NextRequest) {
  if (!GAMING_SERVICE_URL) {
    return NextResponse.json(mockCatalog())
  }

  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/shop/catalog`, {
    headers: { 'Content-Type': 'application/json', ...authHeader(req) },
    cache: 'no-store',
  })

  if (!upstream.ok) {
    return NextResponse.json({ error: 'Failed to fetch shop catalog' }, { status: upstream.status })
  }

  return NextResponse.json(await upstream.json())
}

/**
 * POST /api/gaming/shop — Purchase a power-up from the gem shop.
 *
 * Forwards the buy request to the gaming service. The gaming service
 * atomically deducts gems and updates the caller's power-up inventory.
 *
 * Body: { user_id: string; power_up: PowerUpType }
 * Response: { power_up: PowerUpType; new_count: number; gems_left: number }
 */
export async function POST(req: NextRequest) {
  if (!GAMING_SERVICE_URL) {
    const body = await req.json()
    return NextResponse.json(mockBuy(body.power_up))
  }

  const body = await req.json()

  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/shop/buy`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeader(req) },
    body: JSON.stringify(body),
  })

  if (!upstream.ok) {
    const text = await upstream.text().catch(() => '')
    return NextResponse.json({ error: text || 'Purchase failed' }, { status: upstream.status })
  }

  return NextResponse.json(await upstream.json())
}

// ── Mock data (used when GAMING_SERVICE_URL is not set) ──────────────────────

/** Mock shop catalog for local development. */
function mockCatalog() {
  return {
    items: [
      {
        type: 'shield',
        label: 'Shield',
        icon: '🛡️',
        description: "Blocks one wrong answer's damage for 3 turns.",
        gem_cost: 2,
      },
      {
        type: 'double_damage',
        label: 'Double Damage',
        icon: '⚔️',
        description: 'Doubles damage dealt on correct answers for 2 turns.',
        gem_cost: 3,
      },
      {
        type: 'heal',
        label: 'Heal',
        icon: '💊',
        description: 'Instantly restores 30 HP.',
        gem_cost: 2,
      },
      {
        type: 'critical',
        label: 'Critical Hit',
        icon: '💥',
        description: 'Guarantees a critical hit on your next correct answer.',
        gem_cost: 5,
      },
    ],
  }
}

/** Mock buy response for local development. */
function mockBuy(powerUp: string) {
  return {
    power_up: powerUp,
    new_count: 1,
    gems_left: 8,
  }
}
