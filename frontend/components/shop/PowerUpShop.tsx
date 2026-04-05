'use client'

/**
 * PowerUpShop.tsx
 *
 * Gem-based shop for purchasing power-ups to bring into boss battles.
 * Displays the full catalog with costs, the player's gem balance, and
 * per-type inventory counts. Calls POST /api/gaming/shop to purchase.
 */

import { useCallback, useEffect, useState } from 'react'

/** A single item in the shop catalog returned by GET /api/gaming/shop. */
export interface ShopItem {
  type: 'double_damage' | 'shield' | 'heal' | 'critical'
  label: string
  icon: string
  description: string
  gem_cost: number
}

/** Props for the PowerUpShop component. */
export interface PowerUpShopProps {
  /** Authenticated user ID — passed to the buy request body. */
  userId: string
  /** Starting gem balance, fetched server-side from the gaming profile. */
  initialGems: number
  /** Starting inventory counts keyed by power-up type. */
  initialInventory: Record<string, number>
}

/**
 * PowerUpShop renders the full gem shop UI.
 *
 * Shows the catalog fetched from the API, the player's live gem balance,
 * and inventory counts that update optimistically on purchase.
 */
export default function PowerUpShop({ userId, initialGems, initialInventory }: PowerUpShopProps) {
  const [catalog, setCatalog] = useState<ShopItem[]>([])
  const [gems, setGems] = useState(initialGems)
  const [inventory, setInventory] = useState<Record<string, number>>(initialInventory)
  const [pending, setPending] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [toast, setToast] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  // Fetch catalog on mount.
  useEffect(() => {
    fetch('/api/gaming/shop')
      .then((r) => r.json())
      .then((data) => setCatalog(data.items ?? []))
      .catch(() => setError('Failed to load shop catalog.'))
      .finally(() => setLoading(false))
  }, [])

  const showToast = useCallback((msg: string) => {
    setToast(msg)
    setTimeout(() => setToast(null), 2500)
  }, [])

  const handleBuy = useCallback(
    async (item: ShopItem) => {
      if (gems < item.gem_cost) return
      setPending(item.type)
      setError(null)
      try {
        const resp = await fetch('/api/gaming/shop', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ user_id: userId, power_up: item.type }),
        })
        if (!resp.ok) {
          const body = await resp.json().catch(() => ({ error: 'Purchase failed' }))
          setError(body.error ?? 'Purchase failed')
          return
        }
        const data: { power_up: string; new_count: number; gems_left: number } = await resp.json()
        setGems(data.gems_left)
        setInventory((prev) => ({ ...prev, [data.power_up]: data.new_count }))
        showToast(`${item.icon} ${item.label} added to inventory!`)
      } catch {
        setError('Network error — please try again.')
      } finally {
        setPending(null)
      }
    },
    [gems, userId, showToast],
  )

  return (
    <div className="min-h-screen bg-bg-deep px-4 py-8 flex flex-col items-center">
      <div className="w-full max-w-md">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-lg font-bold text-text-bright font-mono tracking-wider">
            💎 Gem Shop
          </h1>
          <div className="flex items-center gap-1.5 bg-bg-card border border-neon-gold/30 rounded-full px-3 py-1">
            <span className="text-sm">💎</span>
            <span className="text-sm font-mono font-bold text-neon-gold">{gems}</span>
            <span className="text-xs text-text-dim">gems</span>
          </div>
        </div>

        {/* Toast */}
        {toast && (
          <div className="mb-4 text-xs text-center py-2 px-3 rounded-lg bg-neon-green/10 border border-neon-green/30 text-neon-green font-mono animate-pulse">
            {toast}
          </div>
        )}

        {/* Error */}
        {error && (
          <div className="mb-4 text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded px-3 py-2">
            {error}
          </div>
        )}

        {/* Loading skeleton */}
        {loading && (
          <div className="space-y-3">
            {[1, 2, 3, 4].map((i) => (
              <div
                key={i}
                className="h-24 bg-bg-card border border-border-mid rounded-xl animate-pulse"
              />
            ))}
          </div>
        )}

        {/* Catalog */}
        {!loading && (
          <div className="space-y-3">
            {catalog.map((item) => {
              const canAfford = gems >= item.gem_cost
              const count = inventory[item.type] ?? 0
              const isBuying = pending === item.type

              return (
                <div
                  key={item.type}
                  className="bg-bg-card border rounded-xl p-4 flex items-center gap-4 transition-colors"
                  style={{ borderColor: canAfford ? '#00aaff22' : '#22224488' }}
                >
                  {/* Icon */}
                  <span className="text-3xl select-none" aria-hidden>
                    {item.icon}
                  </span>

                  {/* Info */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-0.5">
                      <span className="text-sm font-mono font-semibold text-text-bright">
                        {item.label}
                      </span>
                      {count > 0 && (
                        <span className="text-[10px] font-mono bg-neon-blue/20 text-neon-blue border border-neon-blue/30 rounded-full px-1.5 py-0.5">
                          ×{count}
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-text-dim leading-snug">{item.description}</p>
                  </div>

                  {/* Buy button */}
                  <button
                    onClick={() => handleBuy(item)}
                    disabled={!canAfford || isBuying || pending !== null}
                    aria-label={`Buy ${item.label} for ${item.gem_cost} gems`}
                    className="flex flex-col items-center gap-0.5 px-3 py-2 rounded-lg border text-xs font-mono transition-all
                      disabled:opacity-40 disabled:cursor-not-allowed
                      enabled:hover:bg-neon-blue/10 enabled:active:scale-95"
                    style={{ borderColor: canAfford ? '#00aaff44' : '#333355' }}
                  >
                    {isBuying ? (
                      <span className="text-text-dim animate-pulse">…</span>
                    ) : (
                      <>
                        <span className="text-neon-gold font-bold">{item.gem_cost}💎</span>
                        <span className="text-text-dim">Buy</span>
                      </>
                    )}
                  </button>
                </div>
              )
            })}
          </div>
        )}

        {/* Empty state */}
        {!loading && catalog.length === 0 && !error && (
          <p className="text-xs text-text-dim text-center py-8">Shop is empty — check back soon.</p>
        )}

        {/* Footer hint */}
        <p className="text-[10px] text-text-dim text-center mt-6">
          Power-ups are consumed during boss battles. Earn gems by defeating bosses and completing
          quests.
        </p>
      </div>
    </div>
  )
}
