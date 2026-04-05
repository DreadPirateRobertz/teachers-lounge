'use client'

import { useEffect, useState, useCallback } from 'react'

// ── Types ─────────────────────────────────────────────────────────────────────

interface Achievement {
  id: string
  achievement_type: string
  badge_name: string
  earned_at: string
}

interface AchievementsResponse {
  achievements: Achievement[]
}

interface AchievementGalleryProps {
  userId: string
  /** If provided, the badge with this type gets a "NEW" glow on first render. */
  highlightType?: string
}

// ── Badge icon mapping ────────────────────────────────────────────────────────

const BADGE_ICON: Record<string, string> = {
  boss_the_atom: '⚛️',
  boss_bonding_brothers: '🔗',
  boss_name_lord: '📜',
  boss_the_stereochemist: '🔮',
  boss_the_reactor: '💥',
  boss_algebra_dragon: '🐉',
  boss_grammar_golem: '🗿',
  boss_history_hydra: '🐍',
  boss_science_sphinx: '🦁',
  boss_slayer: '⚔️',
}

function badgeIcon(achievementType: string): string {
  return BADGE_ICON[achievementType] ?? '🏆'
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

// ── Main component ────────────────────────────────────────────────────────────

/**
 * AchievementGallery fetches and displays a user's earned achievement badges.
 * Each badge shows its icon, name, and the date it was earned.
 * The optional highlightType prop causes a new badge to pulse on first render.
 */
export default function AchievementGallery({ userId, highlightType }: AchievementGalleryProps) {
  const [achievements, setAchievements] = useState<Achievement[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await fetch(`/api/gaming/achievements/${encodeURIComponent(userId)}`, {
        cache: 'no-store',
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data: AchievementsResponse = await res.json()
      setAchievements(data.achievements ?? [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load achievements')
    } finally {
      setLoading(false)
    }
  }, [userId])

  useEffect(() => {
    load()
  }, [load])

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="font-mono text-base font-bold text-text-bright tracking-wide">
          Achievement Gallery
        </h2>
        {!loading && !error && (
          <span className="text-xs text-text-dim font-mono">
            {achievements.length} badge{achievements.length !== 1 ? 's' : ''}
          </span>
        )}
      </div>

      {loading ? (
        <GallerySkeleton />
      ) : error ? (
        <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
          {error}
        </div>
      ) : achievements.length === 0 ? (
        <EmptyState />
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {achievements.map((a) => (
            <BadgeCard key={a.id} achievement={a} isNew={a.achievement_type === highlightType} />
          ))}
        </div>
      )}
    </div>
  )
}

// ── Badge card ────────────────────────────────────────────────────────────────

interface BadgeCardProps {
  achievement: Achievement
  isNew: boolean
}

function BadgeCard({ achievement, isNew }: BadgeCardProps) {
  return (
    <div
      className={`flex flex-col items-center gap-2 p-4 rounded-xl border bg-bg-card transition-all ${
        isNew
          ? 'border-neon-gold/50 shadow-neon-gold animate-pulse-slow'
          : 'border-border-mid hover:border-border-mid/60'
      }`}
      aria-label={`${achievement.badge_name} — earned ${formatDate(achievement.earned_at)}`}
    >
      <span className="text-3xl leading-none" aria-hidden="true">
        {badgeIcon(achievement.achievement_type)}
      </span>
      <div className="text-center">
        <div
          className={`font-mono text-[11px] font-bold leading-tight ${
            isNew ? 'text-neon-gold' : 'text-text-bright'
          }`}
        >
          {achievement.badge_name}
        </div>
        <div className="text-[10px] text-text-dim mt-0.5">{formatDate(achievement.earned_at)}</div>
      </div>
      {isNew ? (
        <span className="text-[9px] font-bold text-bg-deep bg-neon-gold px-1.5 py-0.5 rounded-full uppercase tracking-wider">
          NEW
        </span>
      ) : null}
    </div>
  )
}

// ── Empty state ───────────────────────────────────────────────────────────────

function EmptyState() {
  return (
    <div className="flex flex-col items-center gap-3 py-10 bg-bg-card border border-border-dim rounded-xl">
      <span className="text-4xl opacity-30" aria-hidden="true">
        🏆
      </span>
      <p className="text-xs text-text-dim text-center max-w-[180px]">
        Defeat bosses to earn achievement badges
      </p>
    </div>
  )
}

// ── Skeleton loader ───────────────────────────────────────────────────────────

function GallerySkeleton() {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
      {[0, 1, 2, 3].map((i) => (
        <div
          key={i}
          className="flex flex-col items-center gap-2 p-4 rounded-xl border border-border-dim bg-bg-card animate-pulse"
        >
          <div className="w-10 h-10 rounded-full bg-border-dim" />
          <div className="w-20 h-2.5 rounded bg-border-dim" />
          <div className="w-14 h-2 rounded bg-border-dim" />
        </div>
      ))}
    </div>
  )
}
