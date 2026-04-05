'use client'

/**
 * ParentalConsentForm — skeleton form for requesting guardian consent.
 *
 * Collects the guardian's email address and POSTs to
 * `/api/user/parental-consent`.  The backend route is a skeleton that will
 * trigger a consent email once the email-delivery service is wired (tl-5zx).
 *
 * On success the `onSuccess` callback is invoked so the parent page can
 * redirect or show a confirmation message.
 */
import { useState, type FormEvent } from 'react'

interface ParentalConsentFormProps {
  /** ID of the minor user for whom consent is being requested. */
  userId: string
  /** Called after a successful consent request submission. */
  onSuccess: () => void
}

/** Simple email format validator. */
function isValidEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
}

/**
 * Form component that collects a guardian email and submits a parental
 * consent request.
 *
 * @param props - {@link ParentalConsentFormProps}
 */
export default function ParentalConsentForm({ userId, onSuccess }: ParentalConsentFormProps) {
  const [email, setEmail] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  /**
   * Validates the form and POSTs the guardian email to the consent endpoint.
   *
   * @param e - The form submission event.
   */
  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)

    if (!email.trim()) {
      setError('Guardian email is required.')
      return
    }
    if (!isValidEmail(email)) {
      setError('Please enter a valid email address.')
      return
    }

    setPending(true)
    try {
      const res = await fetch('/api/user/parental-consent', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: userId, guardian_email: email }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error ?? 'Failed to send consent request.')
        return
      }
      onSuccess()
    } catch {
      setError('Network error — please try again.')
    } finally {
      setPending(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-4 max-w-sm">
      <p className="text-sm text-text-dim font-mono">
        Enter your parent or guardian&apos;s email address. They will receive a link to approve your
        account.
      </p>

      <div className="flex flex-col gap-1.5">
        <label
          htmlFor="guardian-email"
          className="text-xs font-mono text-text-dim uppercase tracking-wide"
        >
          Guardian Email
        </label>
        <input
          id="guardian-email"
          type="email"
          autoComplete="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="parent@example.com"
          disabled={pending}
          className="rounded-lg px-3 py-2 text-sm font-mono bg-bg-card border border-border-dim
            text-text-base placeholder:text-text-dim focus:outline-none
            focus:border-neon-blue/60 disabled:opacity-50"
        />
      </div>

      {error && (
        <p role="alert" className="text-xs font-mono text-neon-pink">
          {error}
        </p>
      )}

      <button
        type="submit"
        disabled={pending}
        className="px-5 py-2.5 rounded-lg font-mono text-sm font-bold
          bg-neon-blue/10 border border-neon-blue/40 text-neon-blue
          hover:bg-neon-blue/20 transition-colors disabled:opacity-50
          disabled:cursor-not-allowed"
      >
        {pending ? 'Sending…' : 'Send Consent Request'}
      </button>
    </form>
  )
}
