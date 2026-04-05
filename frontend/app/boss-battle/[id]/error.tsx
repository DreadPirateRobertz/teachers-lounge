'use client'

import Link from 'next/link'
import { useEffect } from 'react'

interface ErrorProps {
  error: Error & { digest?: string }
  reset: () => void
}

/**
 * Next.js App Router error boundary for the boss battle route.
 *
 * Catches unhandled errors from the route segment (including future Phase 4
 * WebGL/Three.js client components) and shows a recoverable fallback instead
 * of a blank page.
 */
export default function BossBattleError({ error, reset }: ErrorProps) {
  useEffect(() => {
    console.error('[BossBattleError]', error)
  }, [error])

  return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-bg-deep text-center px-4">
      <div className="text-6xl mb-6">💥</div>

      <h1 className="font-mono text-xl font-bold text-neon-pink text-glow-pink mb-2">
        Boss Battle Crashed
      </h1>
      <p className="text-xs text-text-dim mb-6 max-w-xs">
        Something went wrong in the arena. Your progress has been saved.
      </p>

      <div className="flex gap-3">
        <button
          onClick={reset}
          className="text-xs text-neon-blue border border-neon-blue/30 px-4 py-2 rounded-lg hover:bg-neon-blue/10 transition-colors font-mono"
        >
          Try again
        </button>
        <Link
          href="/"
          className="text-xs text-text-dim border border-border-dim px-4 py-2 rounded-lg hover:bg-bg-card transition-colors font-mono"
        >
          ← Back to Tutor
        </Link>
      </div>
    </div>
  )
}
