/**
 * Adaptive Dashboard page (`/adaptive`).
 *
 * Displays four personalised learning widgets for the authenticated student:
 *   - Mastery Heatmap    – per-concept mastery tiers as a colour-coded grid
 *   - Upcoming Reviews   – SM-2-style spaced repetition schedule
 *   - Weak Concept Alerts – concepts with low accuracy that need attention
 *   - Learning Style Indicator – Felder-Silverman dimension profile
 *
 * All data is fetched client-side from the Next.js API proxy routes which
 * delegate to the analytics-service and user-service respectively.
 */
'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import AppHeader from '@/components/layout/AppHeader'
import XpProgressBar from '@/components/layout/XpProgressBar'

// ── Types ─────────────────────────────────────────────────────────────────────

interface ConceptMastery {
  concept: string
  correct: number
  total: number
  accuracy_pct: number
  mastery_level: 'weak' | 'developing' | 'strong' | 'mastered'
}

interface ReviewItem {
  concept: string
  due_date: string
  days_overdue: number
  priority: 'urgent' | 'soon' | 'upcoming'
}

interface LearningProfile {
  felder_silverman_dials: Record<string, number>
}

// ── Auth helper ───────────────────────────────────────────────────────────────

/** Parse JWT payload without verifying the signature (public claims only). */
function parseJwt(token: string): Record<string, unknown> {
  try {
    return JSON.parse(atob(token.split('.')[1]))
  } catch {
    return {}
  }
}

/** Read the logged-in user's ID from the `tl_token` cookie. */
function getUserIdFromCookie(): string | null {
  if (typeof document === 'undefined') return null
  const match = document.cookie.match(/(?:^|;\s*)tl_token=([^;]+)/)
  if (!match) return null
  const claims = parseJwt(decodeURIComponent(match[1]))
  return (claims.sub as string) ?? null
}

// ── Page ──────────────────────────────────────────────────────────────────────

/**
 * Adaptive Dashboard page component.
 *
 * Fetches mastery, upcoming-reviews, and learning-profile data in parallel
 * once the user ID is resolved from the auth cookie.
 */
export default function AdaptiveDashboardPage() {
  const [mastery, setMastery] = useState<ConceptMastery[]>([])
  const [reviews, setReviews] = useState<ReviewItem[]>([])
  const [profile, setProfile] = useState<LearningProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    const uid = getUserIdFromCookie()
    if (!uid) {
      setError('Not authenticated')
      setLoading(false)
      return
    }

    Promise.all([
      fetch(`/api/analytics/student/${uid}/mastery`).then((r) => r.json()),
      fetch(`/api/analytics/student/${uid}/upcoming-reviews`).then((r) => r.json()),
      fetch(`/api/user/profile/${uid}`).then((r) => r.json()),
    ])
      .then(([masteryData, reviewsData, profileData]) => {
        if (masteryData.error) throw new Error(masteryData.error)
        setMastery((masteryData as { concepts: ConceptMastery[] }).concepts ?? [])
        setReviews((reviewsData as { reviews: ReviewItem[] }).reviews ?? [])
        const lp = (profileData as { learning_profile?: LearningProfile }).learning_profile
        if (lp) setProfile(lp)
      })
      .catch(() => setError('Failed to load adaptive data'))
      .finally(() => setLoading(false))
  }, [])

  const weakConcepts = mastery.filter((c) => c.mastery_level === 'weak')

  return (
    <div className="flex flex-col h-screen bg-bg-deep overflow-hidden">
      <AppHeader />

      <div className="flex-1 overflow-y-auto px-4 py-6 max-w-2xl mx-auto w-full">
        {/* Back nav */}
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-xs text-text-dim hover:text-neon-blue transition-colors mb-6"
        >
          <span>←</span>
          <span>Back to dashboard</span>
        </Link>

        {/* Page title */}
        <div className="mb-6">
          <h1 className="text-lg font-bold text-text-bright font-mono tracking-wide">
            Adaptive Dashboard
          </h1>
          <p className="text-xs text-text-dim mt-0.5">
            Personalised insights — how you learn, what to review next
          </p>
        </div>

        {/* Loading skeleton */}
        {loading && <AdaptiveSkeleton />}

        {/* Error */}
        {error && !loading && (
          <div
            data-testid="adaptive-error"
            className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded px-4 py-3"
          >
            {error}
          </div>
        )}

        {/* Content */}
        {!loading && !error && (
          <div className="flex flex-col gap-5 animate-fade-in" data-testid="adaptive-content">
            {/* Row 1: Mastery Heatmap + Upcoming Reviews */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <MasteryHeatmap concepts={mastery} />
              <UpcomingReviews reviews={reviews} />
            </div>

            {/* Row 2: Weak Concept Alerts */}
            <WeakConceptAlerts concepts={weakConcepts} />

            {/* Row 3: Learning Style Indicator */}
            <LearningStyleIndicator profile={profile} />
          </div>
        )}
      </div>

      <XpProgressBar current={2340} levelMax={3000} level={12} />
    </div>
  )
}

// ── Loading skeleton ──────────────────────────────────────────────────────────

/** Placeholder skeleton shown while adaptive data is loading. */
function AdaptiveSkeleton() {
  return (
    <div className="flex flex-col gap-4" data-testid="adaptive-skeleton">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div className="bg-bg-card border border-border-dim rounded-xl p-5 h-48 animate-pulse" />
        <div className="bg-bg-card border border-border-dim rounded-xl p-5 h-48 animate-pulse" />
      </div>
      <div className="bg-bg-card border border-border-dim rounded-xl p-5 h-24 animate-pulse" />
      <div className="bg-bg-card border border-border-dim rounded-xl p-5 h-40 animate-pulse" />
    </div>
  )
}

// ── Mastery level config ──────────────────────────────────────────────────────

const MASTERY_CONFIG: Record<
  ConceptMastery['mastery_level'],
  { label: string; bg: string; text: string; border: string }
> = {
  weak: { label: 'Weak', bg: 'bg-red-500/20', text: 'text-red-400', border: 'border-red-500/30' },
  developing: {
    label: 'Developing',
    bg: 'bg-yellow-500/15',
    text: 'text-yellow-400',
    border: 'border-yellow-500/30',
  },
  strong: {
    label: 'Strong',
    bg: 'bg-neon-blue/15',
    text: 'text-neon-blue',
    border: 'border-neon-blue/30',
  },
  mastered: {
    label: 'Mastered',
    bg: 'bg-neon-green/15',
    text: 'text-neon-green',
    border: 'border-neon-green/30',
  },
}

// ── MasteryHeatmap ────────────────────────────────────────────────────────────

interface MasteryHeatmapProps {
  /** All concept mastery records for the student. */
  concepts: ConceptMastery[]
}

/**
 * Colour-coded grid of per-concept mastery tiers.
 *
 * Each cell shows the concept name and accuracy percentage, colour-coded by
 * mastery tier (weak → red, developing → yellow, strong → blue, mastered → green).
 */
function MasteryHeatmap({ concepts }: MasteryHeatmapProps) {
  return (
    <section
      className="bg-bg-card border border-border-dim rounded-xl p-4"
      data-testid="mastery-heatmap"
    >
      <h2 className="text-xs font-mono uppercase tracking-widest text-text-dim mb-3">
        🧠 Mastery Heatmap
      </h2>

      {concepts.length === 0 ? (
        <p className="text-xs text-text-dim italic">
          No quiz data yet — start answering questions to see your mastery map.
        </p>
      ) : (
        <div className="grid grid-cols-2 gap-1.5">
          {concepts.map((c) => {
            const cfg = MASTERY_CONFIG[c.mastery_level]
            return (
              <div
                key={c.concept}
                className={`${cfg.bg} border ${cfg.border} rounded-lg px-2.5 py-2`}
                title={`${c.correct}/${c.total} correct`}
              >
                <div className="text-[10px] text-text-dim truncate">{c.concept}</div>
                <div className={`text-xs font-bold font-mono ${cfg.text}`}>{c.accuracy_pct}%</div>
              </div>
            )
          })}
        </div>
      )}

      {/* Legend */}
      {concepts.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-2">
          {(
            Object.entries(MASTERY_CONFIG) as [
              ConceptMastery['mastery_level'],
              (typeof MASTERY_CONFIG)['weak'],
            ][]
          ).map(([level, cfg]) => (
            <div key={level} className="flex items-center gap-1">
              <div className={`w-2 h-2 rounded-full ${cfg.bg} border ${cfg.border}`} />
              <span className="text-[10px] text-text-dim">{cfg.label}</span>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

// ── UpcomingReviews ───────────────────────────────────────────────────────────

interface UpcomingReviewsProps {
  /** Scheduled review items ordered by urgency. */
  reviews: ReviewItem[]
}

/** Badge colour per review priority. */
const PRIORITY_STYLE: Record<ReviewItem['priority'], string> = {
  urgent: 'bg-red-500/20 text-red-400 border-red-500/30',
  soon: 'bg-yellow-500/15 text-yellow-400 border-yellow-500/30',
  upcoming: 'bg-neon-blue/10 text-neon-blue border-neon-blue/20',
}

/**
 * List of upcoming spaced-repetition review sessions.
 *
 * Shows concept name, due date, and priority badge (urgent / soon / upcoming).
 */
function UpcomingReviews({ reviews }: UpcomingReviewsProps) {
  return (
    <section
      className="bg-bg-card border border-border-dim rounded-xl p-4"
      data-testid="upcoming-reviews"
    >
      <h2 className="text-xs font-mono uppercase tracking-widest text-text-dim mb-3">
        📅 Upcoming Reviews
      </h2>

      {reviews.length === 0 ? (
        <p className="text-xs text-text-dim italic">
          No reviews scheduled yet — complete quizzes to build your review queue.
        </p>
      ) : (
        <ul className="space-y-2">
          {reviews.slice(0, 8).map((r) => (
            <li key={r.concept} className="flex items-center justify-between gap-2">
              <span className="text-xs text-text-base truncate flex-1">{r.concept}</span>
              <div className="flex items-center gap-1.5 shrink-0">
                <span className="text-[10px] text-text-dim font-mono">{r.due_date}</span>
                <span
                  className={`text-[9px] font-bold uppercase tracking-wider px-1.5 py-0.5 rounded border ${PRIORITY_STYLE[r.priority]}`}
                >
                  {r.priority === 'urgent' && r.days_overdue > 0
                    ? `${r.days_overdue}d late`
                    : r.priority}
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

// ── WeakConceptAlerts ─────────────────────────────────────────────────────────

interface WeakConceptAlertsProps {
  /** Concepts with 'weak' mastery level. */
  concepts: ConceptMastery[]
}

/**
 * Alert panel highlighting concepts the student struggles with most.
 *
 * Shown only when there are weak concepts; hidden otherwise.
 */
function WeakConceptAlerts({ concepts }: WeakConceptAlertsProps) {
  if (concepts.length === 0) {
    return (
      <section
        className="bg-neon-green/5 border border-neon-green/20 rounded-xl p-4"
        data-testid="weak-concept-alerts"
      >
        <h2 className="text-xs font-mono uppercase tracking-widest text-neon-green mb-1">
          ✅ No Weak Areas
        </h2>
        <p className="text-xs text-text-dim">
          Great job! All attempted concepts are at developing level or higher.
        </p>
      </section>
    )
  }

  return (
    <section
      className="bg-red-500/5 border border-red-500/20 rounded-xl p-4"
      data-testid="weak-concept-alerts"
    >
      <h2 className="text-xs font-mono uppercase tracking-widest text-red-400 mb-3">
        ⚠️ Weak Concepts — Needs Attention
      </h2>
      <div className="flex flex-wrap gap-2">
        {concepts.map((c) => (
          <div
            key={c.concept}
            className="flex items-center gap-1.5 bg-red-500/10 border border-red-500/25 rounded-full px-3 py-1"
          >
            <span className="text-xs text-red-400 font-medium">{c.concept}</span>
            <span className="text-[10px] text-red-500 font-mono">{c.accuracy_pct}%</span>
          </div>
        ))}
      </div>
      <p className="mt-3 text-[11px] text-text-dim">
        Ask Prof Nova to explain these topics in more detail, or try the quiz again to build
        mastery.
      </p>
    </section>
  )
}

// ── LearningStyleIndicator ────────────────────────────────────────────────────

interface LearningStyleIndicatorProps {
  /** Student learning profile from the user-service (may be null if not yet assessed). */
  profile: LearningProfile | null
}

/** Felder-Silverman dimension metadata. */
const DIMENSION_META: Record<
  string,
  { label: string; left: string; right: string; color: string }
> = {
  active_reflective: { label: 'Processing', left: 'Active', right: 'Reflective', color: '#00aaff' },
  sensing_intuitive: { label: 'Perception', left: 'Sensing', right: 'Intuitive', color: '#00ff88' },
  visual_verbal: { label: 'Input', left: 'Visual', right: 'Verbal', color: '#ff00aa' },
  sequential_global: {
    label: 'Understanding',
    left: 'Sequential',
    right: 'Global',
    color: '#ffdc00',
  },
}

const DIMENSION_ORDER = [
  'active_reflective',
  'sensing_intuitive',
  'visual_verbal',
  'sequential_global',
]

/**
 * Felder-Silverman learning style profile visualisation.
 *
 * Renders a bilateral bar chart for each of the four FSLSM dimensions.
 * Prompts the user to complete the assessment if no dials are stored.
 */
function LearningStyleIndicator({ profile }: LearningStyleIndicatorProps) {
  const dials = profile?.felder_silverman_dials ?? {}
  const hasData = Object.keys(dials).length > 0

  return (
    <section
      className="bg-bg-card border border-border-dim rounded-xl p-4"
      data-testid="learning-style-indicator"
    >
      <h2 className="text-xs font-mono uppercase tracking-widest text-text-dim mb-3">
        🎓 Learning Style
      </h2>

      {!hasData ? (
        <div className="text-center py-4">
          <p className="text-xs text-text-dim mb-3">
            You haven&apos;t completed the learning style assessment yet. It takes ~3 minutes and
            helps Professor Nova adapt explanations to how you learn best.
          </p>
          <Link
            href="/assessment"
            className="inline-block px-4 py-2 bg-neon-blue text-bg-deep text-xs font-semibold rounded-lg hover:bg-neon-blue/90 transition-colors"
          >
            Take the Assessment
          </Link>
        </div>
      ) : (
        <div className="space-y-4">
          {DIMENSION_ORDER.map((dim) => {
            const meta = DIMENSION_META[dim]
            if (!meta) return null
            const value = dials[dim] ?? 0
            const leftPct = Math.round(Math.max(0, -value * 50))
            const rightPct = Math.round(Math.max(0, value * 50))
            const isLeft = value < -0.1
            const isRight = value > 0.1
            const isNeutral = !isLeft && !isRight

            return (
              <div key={dim}>
                <div className="flex justify-between items-center mb-1">
                  <span
                    className={`text-[11px] font-medium ${isLeft ? 'text-text-bright' : 'text-text-dim'}`}
                  >
                    {meta.left}
                  </span>
                  <span className="text-[10px] text-text-dim uppercase tracking-wider font-mono">
                    {meta.label}
                  </span>
                  <span
                    className={`text-[11px] font-medium ${isRight ? 'text-text-bright' : 'text-text-dim'}`}
                  >
                    {meta.right}
                  </span>
                </div>

                {/* Track */}
                <div className="relative h-1.5 bg-border-dim rounded-full overflow-hidden">
                  <div
                    className="absolute inset-0 rounded-full transition-all duration-700"
                    style={{
                      left: isLeft ? `${50 - leftPct}%` : '50%',
                      right: isRight ? `${50 - rightPct}%` : '50%',
                      background: meta.color,
                      boxShadow: `0 0 6px ${meta.color}88`,
                    }}
                  />
                  <div className="absolute left-1/2 top-0 bottom-0 w-px bg-border-mid" />
                </div>

                <div className="flex justify-between mt-0.5">
                  <span className="text-[10px] text-text-dim font-mono">
                    {isLeft ? `${leftPct}%` : ''}
                  </span>
                  <span className="text-[10px] text-text-dim">{isNeutral ? 'Balanced' : ''}</span>
                  <span className="text-[10px] text-text-dim font-mono">
                    {isRight ? `${rightPct}%` : ''}
                  </span>
                </div>
              </div>
            )
          })}

          <div className="pt-1 border-t border-border-dim">
            <Link href="/assessment" className="text-[11px] text-neon-blue hover:underline">
              Retake assessment →
            </Link>
          </div>
        </div>
      )}
    </section>
  )
}
