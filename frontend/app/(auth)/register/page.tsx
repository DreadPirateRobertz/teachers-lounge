'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { register } from '@/lib/auth'

export default function RegisterPage() {
  const router = useRouter()

  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    if (password !== confirm) {
      setError('Passwords do not match')
      return
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }
    setLoading(true)
    try {
      await register(email, password, displayName)
      router.push('/subscribe')
      router.refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="w-full max-w-sm">
      <div className="bg-bg-card border border-border-mid rounded-xl p-6 shadow-neon-blue">
        <h1 className="text-lg font-bold text-text-bright mb-1">Create your account</h1>
        <p className="text-xs text-text-dim mb-6">
          Start your free trial — no credit card required
        </p>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Field label="Display Name" htmlFor="display-name">
            <input
              id="display-name"
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="ChemWizard"
              required
              autoComplete="nickname"
              className="neon-input"
            />
          </Field>

          <Field label="Email" htmlFor="email">
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@university.edu"
              required
              autoComplete="email"
              className="neon-input"
            />
          </Field>

          <Field label="Password" htmlFor="password">
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Min. 8 characters"
              required
              autoComplete="new-password"
              className="neon-input"
            />
          </Field>

          <Field label="Confirm Password" htmlFor="confirm">
            <input
              id="confirm"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder="••••••••"
              required
              autoComplete="new-password"
              className="neon-input"
            />
          </Field>

          {error && (
            <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded px-3 py-2">
              {error}
            </div>
          )}

          <button type="submit" disabled={loading} className="neon-btn-primary">
            {loading ? 'Creating account…' : 'Create Account ⚡'}
          </button>

          <p className="text-[10px] text-text-dim text-center leading-relaxed">
            By creating an account you agree to our Terms of Service and Privacy Policy.
            FERPA-compliant data handling.
          </p>
        </form>

        <div className="mt-4 text-center">
          <p className="text-xs text-text-dim">
            Already have an account?{' '}
            <Link href="/login" className="text-neon-blue hover:underline transition-colors">
              Sign in →
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}

function Field({
  label,
  htmlFor,
  children,
}: {
  label: string
  htmlFor: string
  children: React.ReactNode
}) {
  return (
    <div>
      <label htmlFor={htmlFor} className="block text-xs font-medium text-text-dim mb-1.5">
        {label}
      </label>
      {children}
    </div>
  )
}
