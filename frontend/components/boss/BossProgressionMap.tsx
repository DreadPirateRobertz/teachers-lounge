'use client'

/**
 * BossProgressionMap.tsx
 *
 * Visual trail of boss nodes showing the player's progression through all six
 * chemistry bosses. Each node reports one of three states:
 *   - "defeated"  — checkmark, full color, links to battle history
 *   - "current"   — pulsing glow, fight CTA button
 *   - "locked"    — dimmed, padlock icon, no interaction
 *
 * Data is fetched from GET /api/gaming/progression on mount.
 */

import Link from 'next/link'
import { useEffect, useState } from 'react'

/** A single boss node as returned by GET /api/gaming/progression. */
export interface ProgressionNode {
  boss_id: string
  name: string
  topic: string
  tier: number
  victory_xp: number
  primary_color: string
  /** "defeated" | "current" | "locked" */
  state: 'defeated' | 'current' | 'locked'
}

/** Full response shape from GET /api/gaming/progression. */
export interface ProgressionData {
  nodes: ProgressionNode[]
  total_defeated: number
}

/** Props for the BossProgressionMap component. */
export interface BossProgressionMapProps {
  /** Optional override for the API endpoint (useful for testing). */
  apiUrl?: string
}

const STATE_ICON: Record<ProgressionNode['state'], string> = {
  defeated: '✓',
  current: '⚔',
  locked: '🔒',
}

/**
 * BossProgressionMap renders the full six-boss trail.
 *
 * Fetches progression data on mount and renders nodes connected by a
 * vertical neon path. Defeated bosses are fully colored; the current boss
 * pulses; locked bosses are dimmed.
 */
export default function BossProgressionMap({
  apiUrl = '/api/gaming/progression',
}: BossProgressionMapProps) {
  const [data, setData] = useState<ProgressionData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetch(apiUrl, { cache: 'no-store' })
      .then((r) => {
        if (!r.ok) throw new Error(`${r.status}`)
        return r.json() as Promise<ProgressionData>
      })
      .then(setData)
      .catch(() => setError('Failed to load boss progression.'))
      .finally(() => setLoading(false))
  }, [apiUrl])

  if (loading) {
    return (
      <div className="flex flex-col items-center gap-4 py-8" aria-label="Loading boss progression">
        {[1, 2, 3, 4, 5, 6].map((i) => (
          <div
            key={i}
            className="w-64 h-16 bg-bg-card border border-border-mid rounded-xl animate-pulse"
          />
        ))}
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded px-4 py-3">
        {error}
      </div>
    )
  }

  if (!data || data.nodes.length === 0) {
    return <p className="text-xs text-text-dim text-center py-8">No bosses found.</p>
  }

  return (
    <div
      className="flex flex-col items-center gap-0 w-full max-w-sm"
      role="list"
      aria-label="Boss progression trail"
    >
      {data.nodes.map((node, idx) => (
        <BossNode key={node.boss_id} node={node} isLast={idx === data.nodes.length - 1} />
      ))}

      {/* Summary footer */}
      <p className="mt-6 text-xs text-text-dim font-mono">
        <span className="text-neon-gold font-bold">{data.total_defeated}</span>
        {' / '}
        {data.nodes.length} bosses defeated
      </p>
    </div>
  )
}

// ── BossNode ──────────────────────────────────────────────────────────────────

interface BossNodeProps {
  node: ProgressionNode
  isLast: boolean
}

/**
 * BossNode renders a single stop on the progression trail.
 *
 * Defeated and current nodes are interactive; locked nodes are inert.
 */
function BossNode({ node, isLast }: BossNodeProps) {
  const isDefeated = node.state === 'defeated'
  const isCurrent = node.state === 'current'
  const isLocked = node.state === 'locked'

  const borderColor = isLocked ? '#22224488' : node.primary_color
  const glowStyle = isCurrent
    ? { boxShadow: `0 0 16px ${node.primary_color}66, 0 0 4px ${node.primary_color}` }
    : {}

  const inner = (
    <div
      className="w-full flex items-center gap-4 px-4 py-3 rounded-xl border transition-all"
      style={{ borderColor, ...glowStyle, opacity: isLocked ? 0.45 : 1 }}
      role="listitem"
      aria-label={`${node.name} — ${node.state}`}
    >
      {/* State icon circle */}
      <div
        className={`flex-shrink-0 w-9 h-9 rounded-full flex items-center justify-center text-sm font-bold border ${isCurrent ? 'animate-pulse' : ''}`}
        style={{
          borderColor: isLocked ? '#444466' : node.primary_color,
          color: isLocked ? '#666688' : node.primary_color,
          backgroundColor: isDefeated ? `${node.primary_color}22` : 'transparent',
        }}
      >
        {STATE_ICON[node.state]}
      </div>

      {/* Boss info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span
            className="text-xs font-mono font-bold tracking-wider truncate"
            style={{ color: isLocked ? '#666688' : node.primary_color }}
          >
            {node.name}
          </span>
          <span className="text-[10px] text-text-dim font-mono flex-shrink-0">T{node.tier}</span>
        </div>
        <p className="text-[10px] text-text-dim truncate">{node.topic}</p>
      </div>

      {/* XP label or fight CTA */}
      {isCurrent && (
        <span
          className="flex-shrink-0 text-[10px] font-mono font-bold px-2 py-1 rounded border"
          style={{ borderColor: node.primary_color, color: node.primary_color }}
        >
          FIGHT
        </span>
      )}
      {isDefeated && (
        <span className="flex-shrink-0 text-[10px] font-mono text-neon-gold">
          +{node.victory_xp} XP
        </span>
      )}
    </div>
  )

  return (
    <div className="w-full flex flex-col items-center">
      {isCurrent || isDefeated ? (
        <Link
          href={`/boss-battle/${node.boss_id}`}
          className="w-full block hover:scale-[1.01] transition-transform"
          aria-disabled={isLocked}
        >
          {inner}
        </Link>
      ) : (
        inner
      )}

      {/* Connector line between nodes */}
      {!isLast && (
        <div
          className="w-px h-6 my-1"
          style={{ backgroundColor: isDefeated ? node.primary_color : '#22224488' }}
          aria-hidden
        />
      )}
    </div>
  )
}
