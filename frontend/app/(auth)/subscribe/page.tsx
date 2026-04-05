'use client'

import { useState } from 'react'

const PLANS = [
  {
    id: 'monthly',
    name: 'Monthly',
    price: '$19.99',
    period: '/mo',
    note: null,
    highlight: false,
    features: ['Unlimited AI tutoring', 'Progress tracking', 'Boss battles'],
  },
  {
    id: 'quarterly',
    name: 'Semester',
    price: '$49.99',
    period: '/semester',
    note: 'Save 17%',
    highlight: true,
    features: ['Everything in Monthly', 'Priority support', 'Study analytics'],
  },
  {
    id: 'semesterly',
    name: 'Annual',
    price: '$89.99',
    period: '/year',
    note: 'Save 25%',
    highlight: false,
    features: ['Everything in Semester', 'Early access to new features', 'Export reports'],
  },
]

export default function SubscribePage() {
  const [selected, setSelected] = useState('quarterly')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function handleChoosePlan(planId: string) {
    setSelected(planId)
    setLoading(true)
    setError('')
    try {
      const res = await fetch('/api/checkout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ planId }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || 'Failed to create checkout session')
      window.location.href = data.checkoutUrl
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
      setLoading(false)
    }
  }

  return (
    <div className="w-full max-w-lg">
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
          {PLANS.map((plan) => (
            <div
              key={plan.id}
              className={`relative flex flex-col px-4 py-3 rounded-lg border transition-all ${
                selected === plan.id
                  ? 'border-neon-blue/60 bg-neon-blue/10 shadow-neon-blue-sm'
                  : 'border-border-mid bg-bg-panel hover:border-neon-blue/30'
              }`}
            >
              <div className="flex items-center justify-between mb-2">
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
              </div>
              <ul className="text-[11px] text-text-dim space-y-0.5 mb-3">
                {plan.features.map((f) => (
                  <li key={f} className="flex items-center gap-1.5">
                    <span className="text-neon-green">✓</span> {f}
                  </li>
                ))}
              </ul>
              <button
                onClick={() => handleChoosePlan(plan.id)}
                disabled={loading}
                className={`w-full py-2 rounded-lg text-xs font-semibold transition-all ${
                  selected === plan.id
                    ? 'bg-neon-blue text-bg-deep shadow-neon-blue-sm hover:bg-neon-blue/90'
                    : 'border border-neon-blue/40 text-neon-blue hover:bg-neon-blue/10'
                } disabled:opacity-50 disabled:cursor-not-allowed`}
              >
                {loading && selected === plan.id ? 'Redirecting…' : 'Choose Plan ⚡'}
              </button>
              {plan.highlight && (
                <div className="absolute -top-2 right-3 text-[10px] font-bold text-bg-deep bg-neon-green px-2 py-0.5 rounded-full">
                  POPULAR
                </div>
              )}
            </div>
          ))}
        </div>

        {error && (
          <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded px-3 py-2 mb-4">
            {error}
          </div>
        )}

        <p className="text-[10px] text-text-dim text-center">
          Cancel anytime. No charge until trial ends. FERPA-compliant &amp; secure.
        </p>
      </div>
    </div>
  )
}
