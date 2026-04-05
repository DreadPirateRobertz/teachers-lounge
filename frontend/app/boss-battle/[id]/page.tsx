'use client'

// Phase 4 stub — WebGL/Three.js boss battles are built in Phase 4
// This page is the future home of the animated boss fight experience.
//
// What will live here:
//   - Three.js canvas (molecule bosses, physics via Cannon.js/Rapier)
//   - HP bars (student + boss), timer, power-up tray
//   - 5-7 quiz rounds with escalating difficulty
//   - Combo system, loot drops, Weird Science particle effects
//   - Sci-fi quote overlays on hits/misses
//
// Loot reveal and achievement gallery are NOW live (Phase 4 loot system).
// Preview them via the "Preview Victory Screen" button below, or by appending
// ?demo=1 to the URL.

import { useState, useEffect, use } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import LootRevealOverlay, { type LootDrop } from '@/components/layout/LootRevealOverlay'
import AchievementGallery from '@/components/layout/AchievementGallery'

// ── Demo loot — simulates defeating THE ATOM ──────────────────────────────────

const ATOM_DEMO_LOOT: LootDrop = {
  xp_earned: 200,
  gems_earned: 18,
  achievement: {
    id: 'demo-ach',
    achievement_type: 'boss_the_atom',
    badge_name: 'ATOM SMASHER',
    earned_at: new Date().toISOString(),
  },
  cosmetic: {
    key: 'avatar_frame',
    value: 'atomic_ring',
  },
  quote: '"We are all made of star-stuff." — Carl Sagan',
  new_badge: true,
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function BossBattlePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()
  const searchParams = useSearchParams()
  const isDemoMode = searchParams.get('demo') === '1'

  const [loot, setLoot] = useState<LootDrop | null>(null)
  const [showGallery, setShowGallery] = useState(false)
  const [newBadgeType, setNewBadgeType] = useState<string | undefined>()

  // In demo mode, show loot reveal immediately (exit-criteria preview).
  useEffect(() => {
    if (isDemoMode) {
      setLoot(ATOM_DEMO_LOOT)
    }
  }, [isDemoMode])

  function handleClaim() {
    if (loot?.achievement) {
      setNewBadgeType(loot.achievement.achievement_type)
    }
    setLoot(null)
    setShowGallery(true)
  }

  // After claiming, show achievement gallery.
  if (showGallery) {
    return (
      <div className="min-h-screen bg-bg-deep px-4 py-8 flex flex-col items-center">
        <div className="w-full max-w-md">
          <div className="mb-6 text-center">
            <div className="text-5xl mb-2" aria-hidden="true">⚛️</div>
            <h1 className="font-mono text-xl font-bold text-neon-gold">
              THE ATOM Defeated!
            </h1>
            <p className="text-xs text-text-dim mt-1">Rewards claimed</p>
          </div>

          <AchievementGallery userId="demo-user" highlightType={newBadgeType} />

          <Link
            href="/"
            className="block w-full text-center mt-6 py-2.5 bg-neon-blue text-bg-deep text-xs font-semibold rounded-xl hover:brightness-110 transition-all"
          >
            ← Return to Progression Map
          </Link>
        </div>
      </div>
    )
  }

  // Loot overlay renders on top of whatever view is showing.
  return (
    <>
      {/* Main stub view */}
      <div className="flex flex-col items-center justify-center min-h-screen bg-bg-deep text-center px-4">
        <div className="relative mb-8">
          <div className="text-8xl animate-pulse-slow" aria-hidden="true">⚗️</div>
          <div className="absolute inset-0 rounded-full bg-neon-pink/5 blur-2xl" />
        </div>

        <h1 className="font-mono text-2xl font-bold text-neon-pink text-glow-pink mb-2">
          Boss Battle
        </h1>
        <p className="text-xs font-mono text-text-dim mb-1">Chapter {id}</p>

        <div className="mt-6 px-6 py-4 bg-bg-card border border-neon-pink/20 rounded-xl max-w-sm">
          <p className="text-sm text-text-base mb-2">
            ⚔️ <strong className="text-text-bright">The Atom</strong> awaits.
          </p>
          <p className="text-xs text-text-dim leading-relaxed">
            Boss battles arrive in{' '}
            <span className="text-neon-gold font-mono">Phase 4</span>. Finish
            studying your course material — the fight will unlock when you
            reach 60% mastery on this chapter.
          </p>
        </div>

        {/* Victory preview button — demonstrates loot reveal UI */}
        <button
          onClick={() => router.push(`/boss-battle/${id}?demo=1`)}
          className="mt-4 text-xs text-neon-gold border border-neon-gold/30 px-4 py-2 rounded-lg hover:bg-neon-gold/10 transition-colors"
          aria-label="Preview victory loot reveal screen"
        >
          ⚡ Preview Victory Screen
        </button>

        <div className="mt-4 text-xs text-text-dim italic">
          &ldquo;Do. Or do not. There is no try.&rdquo; — Yoda
        </div>

        <Link
          href="/"
          className="mt-8 text-xs text-neon-blue border border-neon-blue/30 px-4 py-2 rounded-lg hover:bg-neon-blue/10 transition-colors"
        >
          ← Back to Tutor
        </Link>
      </div>

      {/* Loot overlay — visible in demo mode */}
      <LootRevealOverlay loot={loot} onClaim={handleClaim} />
    </>
  )
}
