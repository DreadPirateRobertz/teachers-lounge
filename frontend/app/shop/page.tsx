import Link from 'next/link'
import PowerUpShop from '@/components/shop/PowerUpShop'

const GAMING_SERVICE_URL = process.env.GAMING_SERVICE_URL

/**
 * Fetches the gaming profile for the given user ID.
 * Returns gem balance and power-up inventory, or sensible defaults on failure.
 *
 * @param userId - The authenticated user's ID.
 * @returns An object with gems and inventory counts.
 */
async function fetchGamingProfile(
  userId: string,
): Promise<{ gems: number; inventory: Record<string, number> }> {
  if (!GAMING_SERVICE_URL) {
    return { gems: 15, inventory: {} }
  }
  try {
    const resp = await fetch(`${GAMING_SERVICE_URL}/gaming/profile/${userId}`, {
      cache: 'no-store',
    })
    if (!resp.ok) return { gems: 0, inventory: {} }
    const data = await resp.json()
    return {
      gems: data.gems ?? 0,
      inventory: data.power_ups ?? {},
    }
  } catch {
    return { gems: 0, inventory: {} }
  }
}

/**
 * ShopPage — server component that pre-fetches the player's gem balance and
 * power-up inventory, then renders the client-side PowerUpShop.
 *
 * userId is a placeholder until session-cookie auth is wired in Phase 2.
 */
export default async function ShopPage() {
  const userId = 'demo-user'
  const { gems, inventory } = await fetchGamingProfile(userId)

  return (
    <div>
      <div className="px-4 pt-4">
        <Link href="/" className="text-xs text-neon-blue hover:text-glow-blue transition-colors">
          ← Back to tutor
        </Link>
      </div>
      <PowerUpShop userId={userId} initialGems={gems} initialInventory={inventory} />
    </div>
  )
}
