'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'

const PLANS = [
  {
    id: 'monthly',
    name: 'Monthly',
    price: '$19',
    period: '/mo',
    note: null,
    highlight: false,
  },
  {
    id: 'quarterly',
    name: 'Semester',
    price: '$49',
    period: '/semester',
    note: 'Save 14%',
    highlight: true,
  },
  {
    id: 'semesterly',
    name: 'Annual',
    price: '$79',
    period: '/year',
    note: 'Save 30%',
    highlight: false,
  },
]

export default function SubscribePage() {
  const router = useRouter()
  const [selected, setSelected] = useState('quarterly')
  const [loading, setLoading] = useState(false)

  async function handleStartTrial() {
    setLoading(true)
    // Phase 1 stub — Stripe checkout will be wired via User Service billing endpoint
    // POST /api/user/billing/checkout { plan: selected } → Stripe redirect
    await new Promise(r => setTimeout(r, 400)) // simulate
    router.push('/')
    router.refresh()
  }

  function handleSkip() {
    // Trial period — allow entry without payment selection
    router.push('/')
    router.refresh()
  }

  return (
    <div className="w-full max-w-md">
      <div className="bg-bg-card border border-border-mid rounded-xl p-6 shadow-neon-blue">
        <h1 className="text-lg font-bold text-text-bright mb-1">Choose your plan</h1>
        <p className="text-xs text-text-dim mb-2">
          Your free trial is active — no charge until it ends.
        </p>
        <div className="flex items-center gap-2 mb-6 px-3 py-2 bg-neon-green/10 border border-neon-green/20 rounded-lg">
          <span className="text-base">🎉</span>
          <span className="text-xs text-neon-green font-medium">
            14-day free trial included with every plan
          </span>
        </div>

        <div className="flex flex-col gap-3 mb-6">
          {PLANS.map(plan => (
            <button
              key={plan.id}
              onClick={() => setSelected(plan.id)}
              className={`relative flex items-center justify-between px-4 py-3 rounded-lg border text-left transition-all ${
                selected === plan.id
                  ? 'border-neon-blue/60 bg-neon-blue/10 shadow-neon-blue-sm'
                  : 'border-border-mid bg-bg-panel hover:border-border-mid'
              }`}
            >
              <div>
                <div className="text-sm font-medium text-text-bright">{plan.name}</div>
                {plan.note && (
                  <div className="text-[10px] text-neon-green font-mono">{plan.note}</div>
                )}
              </div>
              <div className="text-right">
                <span className="font-mono text-base font-bold text-text-bright">
                  {plan.price}
                </span>
                <span className="text-xs text-text-dim">{plan.period}</span>
              </div>
              {plan.highlight && (
                <div className="absolute -top-2 right-3 text-[10px] font-bold text-bg-deep bg-neon-green px-2 py-0.5 rounded-full">
                  POPULAR
                </div>
              )}
              {selected === plan.id && (
                <div className="absolute left-2 top-1/2 -translate-y-1/2 w-1 h-6 rounded-full bg-neon-blue shadow-neon-blue-sm" />
              )}
            </button>
          ))}
        </div>

        <button
          onClick={handleStartTrial}
          disabled={loading}
          className="neon-btn-primary mb-3"
        >
          {loading ? 'Setting up…' : 'Start Free Trial ⚡'}
        </button>
        <button onClick={handleSkip} className="neon-btn-secondary">
          Skip for now
        </button>

        <p className="text-[10px] text-text-dim text-center mt-4">
          Cancel anytime. No charge until trial ends.
          FERPA-compliant &amp; secure.
        </p>
      </div>
    </div>
  )
}
