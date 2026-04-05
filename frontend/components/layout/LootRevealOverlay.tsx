'use client'

import { useEffect, useState } from 'react'

// ── Types ────────────────────────────────────────────────────────────────────

interface Achievement {
  id: string
  achievement_type: string
  badge_name: string
  earned_at: string
}

interface Cosmetic {
  key: string
  value: string
}

export interface LootDrop {
  xp_earned: number
  gems_earned: number
  achievement?: Achievement
  cosmetic?: Cosmetic
  quote: string
  new_badge: boolean
}

interface LootRevealOverlayProps {
  /** The loot drop to reveal. Pass null/undefined to hide the overlay. */
  loot: LootDrop | null
  /** Called when the student taps "Claim" to collect their rewards. */
  onClaim: () => void
}

// ── Reveal phases ─────────────────────────────────────────────────────────────

type Phase = 'hidden' | 'portal' | 'xp' | 'gems' | 'badge' | 'cosmetic' | 'claim'

const PHASE_DELAYS: Record<Phase, number> = {
  hidden: 0,
  portal: 200,
  xp: 900,
  gems: 1600,
  badge: 2300,
  cosmetic: 3000,
  claim: 3600,
}

const PHASES: Phase[] = ['portal', 'xp', 'gems', 'badge', 'cosmetic', 'claim']

// ── Component ─────────────────────────────────────────────────────────────────

/**
 * LootRevealOverlay displays a full-screen victory animation when the student
 * defeats a boss. Items (XP, gems, badge, cosmetic) appear one by one via
 * staggered CSS animations. The "Claim" button becomes available after all
 * items have revealed.
 */
export default function LootRevealOverlay({ loot, onClaim }: LootRevealOverlayProps) {
  const [phase, setPhase] = useState<Phase>('hidden')

  useEffect(() => {
    if (!loot) {
      setPhase('hidden')
      return
    }

    const timers: ReturnType<typeof setTimeout>[] = []
    for (const p of PHASES) {
      const delay = PHASE_DELAYS[p]
      timers.push(setTimeout(() => setPhase(p), delay))
    }
    return () => timers.forEach(clearTimeout)
  }, [loot])

  if (!loot || phase === 'hidden') return null

  const reached = (p: Phase) => PHASES.indexOf(phase) >= PHASES.indexOf(p)

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Victory! Loot revealed"
      className="fixed inset-0 z-50 flex flex-col items-center justify-center bg-bg-deep/95 backdrop-blur-sm px-4"
    >
      {/* Portal / chest icon */}
      <div
        className={`text-8xl mb-4 transition-all duration-700 ${
          reached('portal') ? 'opacity-100 scale-100' : 'opacity-0 scale-50'
        }`}
        aria-hidden="true"
      >
        🌀
      </div>

      {/* Victory header */}
      <h2
        className={`font-mono text-2xl font-bold text-neon-gold mb-1 transition-all duration-500 ${
          reached('portal') ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-4'
        }`}
      >
        VICTORY!
      </h2>

      {/* Sci-fi quote */}
      <p
        className={`text-[11px] text-text-dim italic text-center max-w-xs mb-6 transition-all duration-500 ${
          reached('portal') ? 'opacity-100' : 'opacity-0'
        }`}
      >
        {loot.quote}
      </p>

      {/* Loot items */}
      <div className="flex flex-col gap-3 w-full max-w-xs">
        {/* XP */}
        <LootItem
          visible={reached('xp')}
          icon="⚡"
          label="XP Earned"
          value={`+${loot.xp_earned.toLocaleString()} XP`}
          colour="neon-blue"
        />

        {/* Gems */}
        <LootItem
          visible={reached('gems')}
          icon="💎"
          label="Gems"
          value={`+${loot.gems_earned} Gems`}
          colour="neon-pink"
        />

        {/* Achievement badge */}
        {loot.achievement ? (
          <LootItem
            visible={reached('badge')}
            icon="🏆"
            label={loot.new_badge ? 'NEW BADGE!' : 'Badge'}
            value={loot.achievement.badge_name}
            colour="neon-gold"
            highlight={loot.new_badge}
          />
        ) : null}

        {/* Cosmetic item */}
        {loot.cosmetic ? (
          <LootItem
            visible={reached('cosmetic')}
            icon="✨"
            label={cosmeticLabel(loot.cosmetic.key)}
            value={loot.cosmetic.value.replace(/_/g, ' ')}
            colour="neon-green"
          />
        ) : null}
      </div>

      {/* Claim button */}
      <button
        onClick={onClaim}
        disabled={!reached('claim')}
        className={`mt-8 w-full max-w-xs py-3 font-mono text-sm font-bold rounded-xl transition-all duration-500 ${
          reached('claim')
            ? 'bg-neon-gold text-bg-deep hover:brightness-110 shadow-neon-gold cursor-pointer'
            : 'bg-bg-card text-text-dim border border-border-dim cursor-not-allowed opacity-50'
        }`}
        aria-label="Claim rewards and continue"
      >
        {reached('claim') ? '⚡ Claim Rewards' : 'Revealing…'}
      </button>
    </div>
  )
}

// ── Helpers ───────────────────────────────────────────────────────────────────

interface LootItemProps {
  visible: boolean
  icon: string
  label: string
  value: string
  colour: 'neon-blue' | 'neon-pink' | 'neon-gold' | 'neon-green'
  highlight?: boolean
}

function LootItem({ visible, icon, label, value, colour, highlight }: LootItemProps) {
  const colourMap = {
    'neon-blue': 'text-neon-blue',
    'neon-pink': 'text-neon-pink',
    'neon-gold': 'text-neon-gold',
    'neon-green': 'text-neon-green',
  }
  const borderMap = {
    'neon-blue': 'border-neon-blue/30',
    'neon-pink': 'border-neon-pink/30',
    'neon-gold': 'border-neon-gold/40',
    'neon-green': 'border-neon-green/30',
  }

  return (
    <div
      className={`flex items-center gap-3 px-4 py-3 rounded-xl border bg-bg-card transition-all duration-500 ${
        visible ? 'opacity-100 translate-x-0' : 'opacity-0 -translate-x-8'
      } ${borderMap[colour]} ${highlight ? 'animate-pulse-slow' : ''}`}
    >
      <span className="text-2xl leading-none flex-shrink-0" aria-hidden="true">
        {icon}
      </span>
      <div className="flex-1 min-w-0">
        <div className="text-[10px] text-text-dim uppercase tracking-wider">{label}</div>
        <div className={`font-mono text-sm font-bold ${colourMap[colour]} truncate`}>{value}</div>
      </div>
    </div>
  )
}

/** Returns a human-readable label for a cosmetic key. */
function cosmeticLabel(key: string): string {
  const labels: Record<string, string> = {
    avatar_frame: 'Avatar Frame',
    color_palette: 'Colour Palette',
    title: 'Title Unlocked',
  }
  return labels[key] ?? key.replace(/_/g, ' ')
}
