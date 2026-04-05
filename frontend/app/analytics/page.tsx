'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import StatCard from '@/components/analytics/StatCard'
import QuizBreakdownChart from '@/components/analytics/QuizBreakdownChart'
import ActivityChart from '@/components/analytics/ActivityChart'
import ErrorBoundary from '@/components/ErrorBoundary'

// ── Types ────────────────────────────────────────────────────────────────────

interface Overview {
  user_id: string
  level: number
  xp: number
  current_streak: number
  longest_streak: number
  total_questions: number
  correct_answers: number
  accuracy_pct: number
  bosses_defeated: number
  gems: number
  total_sessions: number
  total_messages: number
}

interface TopicStat {
  topic: string
  total: number
  correct: number
  accuracy_pct: number
}

interface DayActivity {
  date: string
  messages: number
}

// ── JWT helper (client-side, public claims only) ─────────────────────────────

function parseJwt(token: string): Record<string, unknown> {
  try {
    return JSON.parse(atob(token.split('.')[1]))
  } catch {
    return {}
  }
}

function getUserIdFromCookie(): string | null {
  const match = document.cookie.match(/(?:^|;\s*)tl_token=([^;]+)/)
  if (!match) return null
  const claims = parseJwt(decodeURIComponent(match[1]))
  return (claims.sub as string) ?? null
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function AnalyticsPage() {
  const [userId, setUserId] = useState<string | null>(null)
  const [overview, setOverview] = useState<Overview | null>(null)
  const [topics, setTopics] = useState<TopicStat[]>([])
  const [activity, setActivity] = useState<DayActivity[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    const uid = getUserIdFromCookie()
    if (!uid) {
      setError('Not authenticated')
      setLoading(false)
      return
    }
    setUserId(uid)

    Promise.all([
      fetch(`/api/analytics/student/${uid}/overview`).then((r) => r.json()),
      fetch(`/api/analytics/student/${uid}/quiz-breakdown`).then((r) => r.json()),
      fetch(`/api/analytics/student/${uid}/activity`).then((r) => r.json()),
    ])
      .then(([ov, qb, ac]) => {
        if (ov.error) throw new Error(ov.error)
        setOverview(ov as Overview)
        setTopics((qb as { topics: TopicStat[] }).topics ?? [])
        setActivity((ac as { days: DayActivity[] }).days ?? [])
      })
      .catch(() => setError('Failed to load analytics'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="min-h-screen bg-bg-deep px-4 py-8 overflow-y-auto">
      <div className="max-w-2xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-lg font-bold text-text-bright font-mono tracking-wide">
              Analytics
            </h1>
            <p className="text-xs text-text-dim mt-0.5">Your learning progress at a glance</p>
          </div>
          <Link
            href="/"
            className="text-xs text-neon-blue hover:text-glow-blue transition-colors font-mono"
          >
            ← Back to tutor
          </Link>
        </div>

        {/* Loading skeleton */}
        {loading && (
          <div className="flex flex-col gap-4">
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <div
                  key={i}
                  className="bg-bg-card border border-border-dim rounded-xl p-4 h-20 animate-pulse"
                />
              ))}
            </div>
            <div className="bg-bg-card border border-border-dim rounded-xl p-5 h-48 animate-pulse" />
            <div className="bg-bg-card border border-border-dim rounded-xl p-5 h-32 animate-pulse" />
          </div>
        )}

        {/* Error */}
        {error && !loading && (
          <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded px-4 py-3">
            {error}
          </div>
        )}

        {/* Content */}
        {overview && !loading && (
          <div className="flex flex-col gap-5 animate-fade-in">
            {/* Stat grid */}
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
              <StatCard
                label="Level"
                value={overview.level}
                sub={`${overview.xp.toLocaleString()} XP`}
                color="blue"
              />
              <StatCard
                label="Streak"
                value={`${overview.current_streak}d`}
                sub={`Best: ${overview.longest_streak}d`}
                color="gold"
              />
              <StatCard
                label="Accuracy"
                value={`${overview.accuracy_pct}%`}
                sub={`${overview.correct_answers}/${overview.total_questions} correct`}
                color={
                  overview.accuracy_pct >= 80
                    ? 'green'
                    : overview.accuracy_pct >= 60
                      ? 'gold'
                      : 'pink'
                }
              />
              <StatCard label="Bosses Defeated" value={overview.bosses_defeated} color="pink" />
              <StatCard label="Gems" value={overview.gems.toLocaleString()} color="pink" />
              <StatCard
                label="Sessions"
                value={overview.total_sessions}
                sub={`${overview.total_messages} messages`}
                color="green"
              />
            </div>

            {/* Quiz breakdown */}
            <section className="bg-bg-card border border-border-dim rounded-xl p-5">
              <h2 className="text-xs font-mono uppercase tracking-widest text-text-dim mb-4">
                Quiz Performance by Topic
              </h2>
              <ErrorBoundary componentName="Quiz Breakdown">
                <QuizBreakdownChart topics={topics} />
              </ErrorBoundary>
            </section>

            {/* Activity heatmap */}
            <section className="bg-bg-card border border-border-dim rounded-xl p-5">
              <h2 className="text-xs font-mono uppercase tracking-widest text-text-dim mb-4">
                Activity — Last 30 Days
              </h2>
              <ActivityChart days={activity} />
            </section>
          </div>
        )}
      </div>
    </div>
  )
}
