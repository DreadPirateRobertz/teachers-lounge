'use client'

import { useEffect, useState, useCallback } from 'react'
import Link from 'next/link'

// ── Types ────────────────────────────────────────────────────────────────────

interface AssessmentOption {
  key: string
  text: string
}

interface AssessmentQuestion {
  id: string
  index: number
  total: number
  dimension: string
  stem: string
  options: AssessmentOption[]
}

interface AssessmentSession {
  id: string
  user_id: string
  status: 'active' | 'completed' | 'abandoned'
  current_index: number
  total_questions: number
  xp_earned: number
  results?: Record<string, number>
  started_at: string
  completed_at?: string
}

// ── JWT helper ───────────────────────────────────────────────────────────────

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

// ── Dimension labels ─────────────────────────────────────────────────────────

const DIM_META: Record<string, { label: string; left: string; right: string; color: string }> = {
  active_reflective: {
    label: 'Processing',
    left: 'Active',
    right: 'Reflective',
    color: '#00aaff',
  },
  sensing_intuitive: {
    label: 'Perception',
    left: 'Sensing',
    right: 'Intuitive',
    color: '#00ff88',
  },
  visual_verbal: {
    label: 'Input',
    left: 'Visual',
    right: 'Verbal',
    color: '#ff00aa',
  },
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

// ── Main page ────────────────────────────────────────────────────────────────

type Phase = 'loading' | 'intro' | 'question' | 'completed' | 'error'

export default function AssessmentPage() {
  const [phase, setPhase] = useState<Phase>('loading')
  const [userId, setUserId] = useState<string | null>(null)
  const [session, setSession] = useState<AssessmentSession | null>(null)
  const [question, setQuestion] = useState<AssessmentQuestion | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => {
    const id = getUserIdFromCookie()
    setUserId(id)
    setPhase(id ? 'intro' : 'error')
    if (!id) setErrorMsg('Not signed in. Please log in first.')
  }, [])

  const startAssessment = useCallback(async () => {
    if (!userId) return
    setPhase('loading')
    try {
      const res = await fetch('/api/assessment', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: userId }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      setSession(data.session)
      setQuestion(data.question)
      setSelected(null)
      setPhase('question')
    } catch {
      setErrorMsg('Failed to start assessment. Please try again.')
      setPhase('error')
    }
  }, [userId])

  const submitAnswer = useCallback(async () => {
    if (!session || !question || !selected || submitting) return
    setSubmitting(true)
    try {
      const res = await fetch(`/api/assessment/sessions/${session.id}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_id: userId,
          question_id: question.id,
          chosen_key: selected,
        }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      setSession(data.session)
      if (data.session.status === 'completed') {
        // Persist dials to user-service learning profile.
        if (data.session.results) {
          await saveDialsToProfile(userId!, data.session.results)
        }
        setPhase('completed')
      } else {
        setQuestion(data.next_question)
        setSelected(null)
      }
    } catch {
      setErrorMsg('Failed to submit answer. Please try again.')
    } finally {
      setSubmitting(false)
    }
  }, [session, question, selected, submitting, userId])

  return (
    <div className="min-h-screen bg-bg-deep px-4 py-8 flex flex-col items-center">
      <div className="w-full max-w-xl">
        <div className="flex items-center justify-between mb-8">
          <Link href="/" className="text-xs text-text-dim hover:text-neon-blue transition-colors">
            ← Back
          </Link>
          <h1 className="text-sm font-semibold text-text-bright tracking-wide uppercase">
            Learning Style Assessment
          </h1>
          <div className="w-16" />
        </div>

        {phase === 'loading' && <LoadingCard />}
        {phase === 'intro' && <IntroCard onStart={startAssessment} />}
        {phase === 'question' && session && question && (
          <QuestionCard
            question={question}
            selected={selected}
            submitting={submitting}
            onSelect={setSelected}
            onSubmit={submitAnswer}
          />
        )}
        {phase === 'completed' && session?.results && (
          <ResultsCard results={session.results} xpEarned={session.xp_earned} />
        )}
        {phase === 'error' && (
          <div className="bg-bg-card border border-red-500/30 rounded-xl p-6 text-center">
            <p className="text-sm text-red-400 mb-4">{errorMsg}</p>
            <Link href="/" className="text-xs text-neon-blue hover:underline">
              Go to dashboard
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}

// ── Sub-components ───────────────────────────────────────────────────────────

function LoadingCard() {
  return (
    <div className="bg-bg-card border border-border-mid rounded-xl p-8">
      <div className="space-y-3">
        <div className="h-4 bg-bg-panel rounded animate-pulse w-2/3 mx-auto" />
        <div className="h-3 bg-bg-panel rounded animate-pulse w-1/2 mx-auto" />
      </div>
    </div>
  )
}

function IntroCard({ onStart }: { onStart: () => void }) {
  return (
    <div className="bg-bg-card border border-neon-blue/25 rounded-xl p-8 animate-fade-in">
      <div className="text-center mb-6">
        <div className="text-3xl mb-3">🧠</div>
        <h2 className="text-base font-semibold text-text-bright mb-2">
          Discover Your Learning Style
        </h2>
        <p className="text-xs text-text-dim leading-relaxed">
          12 quick questions help Professor Nova understand how you learn best — visual or verbal,
          active or reflective, sensing or intuitive, sequential or global. Your answers shape how
          Nova explains things to you.
        </p>
      </div>

      <div className="grid grid-cols-2 gap-3 mb-6 text-[11px] text-text-dim">
        {[
          { icon: '⚡', label: '~3 minutes' },
          { icon: '🎯', label: 'Personalises Nova' },
          { icon: '🏆', label: '+100 XP reward' },
          { icon: '🔄', label: 'Retake anytime' },
        ].map(({ icon, label }) => (
          <div key={label} className="flex items-center gap-2 bg-bg-panel rounded-lg px-3 py-2">
            <span>{icon}</span>
            <span>{label}</span>
          </div>
        ))}
      </div>

      <button
        onClick={onStart}
        className="w-full py-3 bg-neon-blue text-bg-deep text-sm font-semibold rounded-xl hover:bg-neon-blue/90 transition-colors shadow-neon-blue"
      >
        Start Assessment
      </button>
    </div>
  )
}

function QuestionCard({
  question,
  selected,
  submitting,
  onSelect,
  onSubmit,
}: {
  question: AssessmentQuestion
  selected: string | null
  submitting: boolean
  onSelect: (key: string) => void
  onSubmit: () => void
}) {
  const progress = (question.index / question.total) * 100

  return (
    <div className="bg-bg-card border border-border-mid rounded-xl p-6 animate-slide-up">
      {/* Progress */}
      <div className="mb-5">
        <div className="flex justify-between text-[10px] text-text-dim mb-1.5">
          <span>
            Question {question.index + 1} of {question.total}
          </span>
          <span>{Math.round(progress)}%</span>
        </div>
        <div className="h-1 bg-border-dim rounded-full overflow-hidden">
          <div
            className="h-full rounded-full transition-all duration-500"
            style={{
              width: `${progress}%`,
              background: 'linear-gradient(90deg, #00aaff, #00ff88)',
              boxShadow: '0 0 6px #00aaff88',
            }}
          />
        </div>
      </div>

      {/* Stem */}
      <p className="text-sm text-text-bright font-medium mb-5 leading-relaxed">{question.stem}</p>

      {/* Options */}
      <div className="space-y-3 mb-6">
        {question.options.map((opt) => {
          const isSelected = selected === opt.key
          return (
            <button
              key={opt.key}
              onClick={() => onSelect(opt.key)}
              className={`w-full text-left px-4 py-3.5 rounded-xl border text-xs leading-relaxed transition-all ${
                isSelected
                  ? 'border-neon-blue/60 bg-neon-blue/10 text-text-bright shadow-neon-blue-sm'
                  : 'border-border-mid bg-bg-panel text-text-base hover:border-border-mid/80 hover:bg-bg-input'
              }`}
            >
              <span
                className={`font-mono font-bold mr-2 ${isSelected ? 'text-neon-blue' : 'text-text-dim'}`}
              >
                {opt.key}
              </span>
              {opt.text}
            </button>
          )
        })}
      </div>

      {/* Next */}
      <button
        onClick={onSubmit}
        disabled={!selected || submitting}
        className="w-full py-2.5 bg-neon-blue text-bg-deep text-xs font-semibold rounded-xl hover:bg-neon-blue/90 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
      >
        {submitting ? 'Saving…' : question.index + 1 === question.total ? 'Finish' : 'Next'}
      </button>
    </div>
  )
}

function ResultsCard({ results, xpEarned }: { results: Record<string, number>; xpEarned: number }) {
  return (
    <div className="space-y-4 animate-fade-in">
      {/* Header */}
      <div className="bg-bg-card border border-neon-green/30 rounded-xl p-6 text-center shadow-neon-green-sm">
        <div className="text-3xl mb-2">✨</div>
        <h2 className="text-base font-semibold text-text-bright mb-1">Assessment Complete!</h2>
        <p className="text-xs text-text-dim mb-3">
          Professor Nova will now adapt explanations to your style.
        </p>
        {xpEarned > 0 && (
          <div className="inline-flex items-center gap-1.5 bg-neon-gold/10 border border-neon-gold/30 rounded-full px-3 py-1">
            <span className="text-neon-gold text-xs font-bold font-mono">+{xpEarned} XP</span>
          </div>
        )}
      </div>

      {/* Dials */}
      <div className="bg-bg-card border border-border-mid rounded-xl p-6 space-y-5">
        <h3 className="text-xs font-semibold text-text-dim uppercase tracking-wider">
          Your Felder-Silverman Profile
        </h3>
        {DIMENSION_ORDER.map((dim) => {
          const meta = DIM_META[dim]
          const value = results[dim] ?? 0
          const leftPct = Math.round(Math.max(0, -value * 50))
          const rightPct = Math.round(Math.max(0, value * 50))
          const isLeft = value < -0.1
          const isRight = value > 0.1
          const isNeutral = !isLeft && !isRight

          return (
            <div key={dim}>
              <div className="flex justify-between items-center mb-1.5">
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
              <div className="relative h-2 bg-border-dim rounded-full overflow-hidden">
                <div
                  className="absolute inset-0 rounded-full transition-all duration-700"
                  style={{
                    left: isLeft ? `${50 - leftPct}%` : '50%',
                    right: isRight ? `${50 - rightPct}%` : '50%',
                    background: meta.color,
                    boxShadow: `0 0 6px ${meta.color}88`,
                  }}
                />
                {/* Centre marker */}
                <div className="absolute left-1/2 top-0 bottom-0 w-px bg-border-mid" />
              </div>

              <div className="flex justify-between mt-1">
                <span className="text-[10px] text-text-dim font-mono">
                  {isLeft ? `${leftPct}% ${meta.left}` : ''}
                </span>
                <span className="text-[10px] text-text-dim">{isNeutral && 'Balanced'}</span>
                <span className="text-[10px] text-text-dim font-mono">
                  {isRight ? `${rightPct}% ${meta.right}` : ''}
                </span>
              </div>
            </div>
          )
        })}
      </div>

      {/* CTA */}
      <Link
        href="/"
        className="block w-full py-3 text-center bg-neon-blue text-bg-deep text-sm font-semibold rounded-xl hover:bg-neon-blue/90 transition-colors shadow-neon-blue"
      >
        Start learning with Nova ⚡
      </Link>
    </div>
  )
}

// ── Save dials to user profile ───────────────────────────────────────────────

async function saveDialsToProfile(userId: string, dials: Record<string, number>) {
  try {
    await fetch(`/api/user/profile/${userId}/preferences`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ felder_silverman_dials: dials }),
    })
  } catch {
    // Non-fatal — assessment results are already stored in the gaming-service session.
  }
}
