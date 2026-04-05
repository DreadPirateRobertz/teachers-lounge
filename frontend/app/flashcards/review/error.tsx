'use client'

import Link from 'next/link'
import { useEffect } from 'react'

interface ErrorProps {
  error: Error & { digest?: string }
  reset: () => void
}

/**
 * Next.js App Router error boundary for the flashcard review session route.
 *
 * Catches unhandled render errors during an active SM-2 review session and
 * displays a recoverable fallback. The user's ratings already submitted are
 * persisted server-side, so partial progress is not lost.
 *
 * @param error - The thrown error, optionally including a digest for
 *   server-side error correlation.
 * @param reset - Callback that retries rendering the failed segment.
 */
export default function FlashcardsReviewError({ error, reset }: ErrorProps) {
  useEffect(() => {
    console.error('[FlashcardsReviewError]', error)
  }, [error])

  return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-bg-deep text-center px-4">
      <div className="text-6xl mb-6">📚</div>

      <h1 className="font-mono text-xl font-bold text-neon-pink text-glow-pink mb-2">
        Review Session Crashed
      </h1>
      <p className="text-xs text-text-dim mb-6 max-w-xs">
        Something went wrong during your review. Ratings you already submitted are saved.
      </p>

      <div className="flex gap-3">
        <button
          onClick={reset}
          className="text-xs text-neon-blue border border-neon-blue/30 px-4 py-2 rounded-lg hover:bg-neon-blue/10 transition-colors font-mono"
        >
          Try again
        </button>
        <Link
          href="/flashcards"
          className="text-xs text-text-dim border border-border-dim px-4 py-2 rounded-lg hover:bg-bg-card transition-colors font-mono"
        >
          ← Back to Deck
        </Link>
      </div>
    </div>
  )
}
