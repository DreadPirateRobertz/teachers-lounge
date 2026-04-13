'use client'

/**
 * LootReveal.tsx (tl-dye)
 *
 * Post-victory loot reveal panel. Staggers in each reward row with a short
 * delay so the player sees each prize "drop" individually. Purely presentational
 * — the parent passes the awarded loot and an `onContinue` callback that is
 * invoked after all rows have animated in.
 */

import { useEffect, useState } from 'react'

/** A single loot line item. */
export interface LootItem {
  /** Stable key for React list rendering and test targeting. */
  key: string
  /** Icon glyph or emoji shown to the left of the label. */
  icon: string
  /** Display label (e.g. "Gems", "XP", "Badge: Atom Slayer"). */
  label: string
  /** Numeric amount, or null for purely qualitative rewards (badges, etc.). */
  amount: number | null
}

export interface LootRevealProps {
  /** Rewards to show. Order controls the reveal sequence. */
  items: LootItem[]
  /** Called once after the final item has revealed. Optional. */
  onContinue?: () => void
  /** Ms between each row appearing. Defaults to 400 — lower for tests. */
  staggerMs?: number
}

const DEFAULT_STAGGER_MS = 400

/**
 * LootReveal renders a staggered reveal of post-battle rewards.
 */
export default function LootReveal({
  items,
  onContinue,
  staggerMs = DEFAULT_STAGGER_MS,
}: LootRevealProps) {
  const [revealed, setRevealed] = useState(0)

  useEffect(() => {
    if (revealed >= items.length) {
      onContinue?.()
      return
    }
    const timer = setTimeout(() => setRevealed((n) => n + 1), staggerMs)
    return () => clearTimeout(timer)
  }, [revealed, items.length, staggerMs, onContinue])

  return (
    <div
      role="region"
      aria-label="Battle rewards"
      className="flex flex-col gap-2 w-full max-w-sm mx-auto px-4 py-6 rounded-lg border border-neon-gold/40 bg-bg-panel/60"
    >
      <h2 className="text-sm font-mono font-bold text-neon-gold tracking-wider text-center mb-2">
        LOOT REVEALED
      </h2>
      {items.map((item, idx) => {
        const visible = idx < revealed
        return (
          <div
            key={item.key}
            data-testid={`loot-row-${item.key}`}
            aria-hidden={!visible}
            className={`flex items-center gap-3 px-3 py-2 rounded-md border transition-all duration-300 ${
              visible
                ? 'opacity-100 translate-y-0 border-neon-gold/30 bg-neon-gold/5'
                : 'opacity-0 translate-y-2 border-transparent'
            }`}
          >
            <span aria-hidden="true" className="text-lg">
              {item.icon}
            </span>
            <span className="text-xs font-mono text-text-bright flex-1">{item.label}</span>
            {item.amount !== null && (
              <span className="text-xs font-mono font-bold text-neon-gold">
                +{item.amount}
              </span>
            )}
          </div>
        )
      })}
    </div>
  )
}
