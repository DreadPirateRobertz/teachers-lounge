'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { login } from '@/lib/auth'

export default function LoginPage() {
  const router = useRouter()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await login(email, password)
      router.push('/')
      router.refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="w-full max-w-sm">
      <div className="bg-bg-card border border-border-mid rounded-xl p-6 shadow-neon-blue">
        <h1 className="text-lg font-bold text-text-bright mb-1">Welcome back</h1>
        <p className="text-xs text-text-dim mb-6">Sign in to continue with Prof. Nova</p>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
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
              placeholder="••••••••"
              required
              autoComplete="current-password"
              className="neon-input"
            />
          </Field>

          {error && (
            <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded px-3 py-2">
              {error}
            </div>
          )}

          <button type="submit" disabled={loading} className="neon-btn-primary">
            {loading ? 'Signing in…' : 'Sign In ⚡'}
          </button>
        </form>

        <div className="mt-4 text-center">
          <p className="text-xs text-text-dim">
            No account?{' '}
            <Link
              href="/register"
              className="text-neon-blue hover:text-glow-blue transition-colors"
            >
              Start your free trial →
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
