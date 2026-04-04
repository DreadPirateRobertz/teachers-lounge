'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'

type SubStatus = 'active' | 'trialing' | 'canceled' | 'past_due' | 'none'

interface Subscription {
  status: SubStatus
  plan_id: string | null
  current_period_end: string | null
  trial_end: string | null
  cancel_at_period_end: boolean
}

const PLAN_LABELS: Record<string, string> = {
  monthly: 'Monthly',
  quarterly: 'Semester',
  semesterly: 'Annual',
}

function daysUntil(dateStr: string): number {
  const ms = new Date(dateStr).getTime() - Date.now()
  return Math.max(0, Math.ceil(ms / (1000 * 60 * 60 * 24)))
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  })
}

export default function ProfilePage() {
  const [sub, setSub] = useState<Subscription | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    fetch('/api/subscription')
      .then(r => r.json())
      .then(data => setSub(data))
      .catch(() => setError('Failed to load subscription'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="min-h-screen bg-bg-deep px-4 py-8 flex flex-col items-center">
      <div className="w-full max-w-md">
        <h1 className="text-lg font-bold text-text-bright mb-6">Account & Subscription</h1>

        {loading && (
          <div className="bg-bg-card border border-border-mid rounded-xl p-6">
            <div className="h-4 bg-bg-panel rounded animate-pulse w-1/2 mb-3" />
            <div className="h-3 bg-bg-panel rounded animate-pulse w-3/4" />
          </div>
        )}

        {error && (
          <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded px-4 py-3">
            {error}
          </div>
        )}

        {sub && !loading && <SubCard sub={sub} />}

        <div className="mt-4 space-y-2">
          <Link
            href="/assessment"
            className="flex items-center justify-between w-full px-4 py-3 bg-bg-card border border-border-mid rounded-xl hover:border-neon-blue/40 transition-colors group"
          >
            <div className="flex items-center gap-2">
              <span className="text-base">🧠</span>
              <div>
                <div className="text-xs font-medium text-text-bright">Learning Style Assessment</div>
                <div className="text-[10px] text-text-dim">Help Nova adapt to how you learn</div>
              </div>
            </div>
            <span className="text-text-dim text-xs group-hover:text-neon-blue transition-colors">→</span>
          </Link>

          <Link href="/" className="block text-xs text-neon-blue hover:text-glow-blue transition-colors text-center py-1">
            ← Back to tutor
          </Link>
        </div>
      </div>
    </div>
  )
}

function SubCard({ sub }: { sub: Subscription }) {
  const planLabel = sub.plan_id ? (PLAN_LABELS[sub.plan_id] ?? sub.plan_id) : null

  if (sub.status === 'trialing' && sub.trial_end) {
    const days = daysUntil(sub.trial_end)
    return (
      <div className="bg-bg-card border border-neon-green/30 rounded-xl p-6 shadow-neon-green-sm">
        <div className="flex items-center justify-between mb-4">
          <span className="text-sm font-semibold text-text-bright">Free Trial</span>
          <span className="text-[10px] font-bold text-bg-deep bg-neon-green px-2 py-0.5 rounded-full">
            ACTIVE
          </span>
        </div>
        <p className="text-2xl font-mono font-bold text-neon-green mb-1">
          {days} <span className="text-sm font-normal text-text-dim">days left</span>
        </p>
        <p className="text-xs text-text-dim mb-4">
          Trial ends {formatDate(sub.trial_end)}
        </p>
        {planLabel && (
          <p className="text-xs text-text-dim mb-4">
            Plan selected: <span className="text-text-base">{planLabel}</span>
          </p>
        )}
        <Link href="/subscribe" className="block w-full text-center py-2 bg-neon-blue text-bg-deep text-xs font-semibold rounded-lg hover:bg-neon-blue/90 transition-colors">
          Upgrade now ⚡
        </Link>
      </div>
    )
  }

  if (sub.status === 'active') {
    return (
      <div className="bg-bg-card border border-neon-blue/30 rounded-xl p-6 shadow-neon-blue">
        <div className="flex items-center justify-between mb-4">
          <span className="text-sm font-semibold text-text-bright">
            {planLabel ?? 'Subscription'}
          </span>
          <span className="text-[10px] font-bold text-bg-deep bg-neon-blue px-2 py-0.5 rounded-full">
            ACTIVE
          </span>
        </div>
        {sub.current_period_end && (
          <p className="text-xs text-text-dim mb-4">
            {sub.cancel_at_period_end
              ? `Cancels on ${formatDate(sub.current_period_end)}`
              : `Renews on ${formatDate(sub.current_period_end)}`}
          </p>
        )}
        {!sub.cancel_at_period_end && (
          <button
            onClick={() => {
              // cancel flow — future implementation
              alert('To cancel, email support@teacherslounge.ai')
            }}
            className="text-xs text-text-dim border border-border-mid px-3 py-1.5 rounded-lg hover:border-red-500/40 hover:text-red-400 transition-colors"
          >
            Cancel subscription
          </button>
        )}
      </div>
    )
  }

  // canceled / past_due / none
  return (
    <div className="bg-bg-card border border-border-mid rounded-xl p-6">
      <div className="flex items-center justify-between mb-4">
        <span className="text-sm font-semibold text-text-bright">No active subscription</span>
        <span className="text-[10px] font-bold text-text-dim border border-border-mid px-2 py-0.5 rounded-full uppercase">
          {sub.status}
        </span>
      </div>
      <p className="text-xs text-text-dim mb-4">
        Subscribe to unlock unlimited AI tutoring and boss battles.
      </p>
      <Link href="/subscribe" className="block w-full text-center py-2 bg-neon-blue text-bg-deep text-xs font-semibold rounded-lg hover:bg-neon-blue/90 transition-colors">
        Subscribe ⚡
      </Link>
    </div>
  )
}
