'use client'

import { useEffect, useState, useCallback } from 'react'
import Link from 'next/link'
import FlashCard from '@/components/flashcard/FlashCard'
import type { Flashcard } from '@/components/flashcard/FlashCard'

// ── JWT helper ───────────────────────────────────────────────────────────────

/**
 * Decodes the payload of a JWT token without verification.
 * Returns an empty object if parsing fails.
 */
function parseJwt(token: string): Record<string, unknown> {
  try {
    return JSON.parse(atob(token.split('.')[1]))
  } catch {
    return {}
  }
}

/**
 * Reads the tl_token cookie from document.cookie and extracts the `sub` claim
 * as the user ID. Returns null if the cookie is absent or invalid.
 */
function getUserIdFromCookie(): string | null {
  const match = document.cookie.match(/(?:^|;\s*)tl_token=([^;]+)/)
  if (!match) return null
  const claims = parseJwt(decodeURIComponent(match[1]))
  return (claims.sub as string) ?? null
}

// ── Types ────────────────────────────────────────────────────────────────────

/** SM-2 quality rating. */
type Quality = 0 | 1 | 2 | 3 | 4 | 5

/** Response shape from GET /api/flashcards/due */
interface DueResponse {
  cards: Flashcard[]
  due_count: number
  total: number
}

// ── Loading skeleton ─────────────────────────────────────────────────────────

/** Skeleton shown while due cards are being fetched. */
function LoadingSkeleton() {
  return (
    <div className="space-y-4 animate-pulse">
      <div className="bg-bg-card border border-border-dim rounded-xl" style={{ height: '220px' }}>
        <div className="flex items-center justify-center h-full">
          <div className="h-4 w-1/2 bg-bg-panel rounded" />
        </div>
      </div>
      <div className="grid grid-cols-6 gap-1">
        {[0, 1, 2, 3, 4, 5].map((i) => (
          <div key={i} className="h-12 bg-bg-panel rounded-lg border border-border-dim" />
        ))}
      </div>
    </div>
  )
}

// ── Main page ────────────────────────────────────────────────────────────────

/**
 * Flashcard review page (/flashcards/review).
 *
 * Loads cards due for review, then presents them one at a time using the
 * FlashCard component. When the user rates a card the SM-2 review is
 * submitted to the API and the session advances to the next card.
 * When all cards are done a completion screen is shown.
 */
export default function ReviewPage() {
  const [userId, setUserId] = useState<string | null>(null)
  const [cards, setCards] = useState<Flashcard[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentIndex, setCurrentIndex] = useState(0)
  const [isRevealed, setIsRevealed] = useState(false)
  const [reviewedCount, setReviewedCount] = useState(0)
  const [done, setDone] = useState(false)
  const [rateError, setRateError] = useState<string | null>(null)

  /** Load due cards on mount. */
  const loadDueCards = useCallback(async () => {
    const uid = getUserIdFromCookie()
    setUserId(uid)
    try {
      const res = await fetch('/api/flashcards/due', { cache: 'no-store' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data: DueResponse = await res.json()
      setCards(data.cards ?? [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load due cards')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadDueCards()
  }, [loadDueCards])

  /**
   * Submits a quality rating for the current card, then advances to the next.
   * Errors are non-blocking — session always advances even if the request fails.
   */
  const handleRate = useCallback(
    async (quality: Quality) => {
      const card = cards[currentIndex]
      if (!card) return

      // Optimistically advance session
      const nextIndex = currentIndex + 1
      const newReviewedCount = reviewedCount + 1

      setReviewedCount(newReviewedCount)
      setRateError(null)

      if (nextIndex >= cards.length) {
        setDone(true)
      } else {
        setCurrentIndex(nextIndex)
        setIsRevealed(false)
      }

      // Submit to backend (non-blocking)
      try {
        const res = await fetch(`/api/flashcards/${card.id}/review`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ user_id: userId, quality }),
        })
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
      } catch (e) {
        // Show a transient error but don't block the user
        setRateError(e instanceof Error ? e.message : 'Review submission failed')
      }
    },
    [cards, currentIndex, reviewedCount, userId],
  )

  /** Handles the reveal action. */
  const handleReveal = useCallback(() => {
    setIsRevealed(true)
  }, [])

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <div className="min-h-screen bg-bg-deep px-4 py-8 flex flex-col items-center">
      <div className="w-full max-w-xl">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <Link
            href="/flashcards"
            className="text-xs text-text-dim hover:text-neon-blue transition-colors"
          >
            ← Deck
          </Link>
          <h1 className="text-sm font-semibold text-text-bright tracking-wide uppercase">Review</h1>
          <div className="w-16" />
        </div>

        {/* Loading */}
        {loading && <LoadingSkeleton />}

        {/* Load error */}
        {error && (
          <div className="bg-bg-card border border-red-500/30 rounded-xl p-6 text-center">
            <p className="text-sm text-red-400 mb-4">{error}</p>
            <Link href="/flashcards" className="text-xs text-neon-blue hover:underline">
              Back to deck
            </Link>
          </div>
        )}

        {/* All caught up */}
        {!loading && !error && cards.length === 0 && (
          <div className="bg-bg-card border border-neon-green/30 rounded-xl p-8 text-center animate-fade-in shadow-neon-green">
            <div className="text-3xl mb-3">✅</div>
            <h2 className="text-base font-semibold text-text-bright mb-2">All caught up!</h2>
            <p className="text-xs text-text-dim mb-5">Come back tomorrow for your next review.</p>
            <Link
              href="/flashcards"
              className="inline-block px-6 py-2.5 bg-neon-blue text-bg-deep text-xs font-semibold rounded-xl hover:bg-neon-blue/90 transition-colors shadow-neon-blue"
            >
              Back to deck
            </Link>
          </div>
        )}

        {/* Completion screen */}
        {done && (
          <div className="bg-bg-card border border-neon-green/30 rounded-xl p-8 text-center animate-fade-in shadow-neon-green">
            <div className="text-3xl mb-3">🎉</div>
            <h2 className="text-base font-semibold text-text-bright mb-2">Session complete!</h2>
            <p className="text-xs text-text-dim mb-5">
              Reviewed <span className="font-mono font-bold text-neon-green">{reviewedCount}</span>{' '}
              {reviewedCount === 1 ? 'card' : 'cards'}.
            </p>
            <Link
              href="/flashcards"
              className="inline-block px-6 py-2.5 bg-neon-blue text-bg-deep text-xs font-semibold rounded-xl hover:bg-neon-blue/90 transition-colors shadow-neon-blue"
            >
              Back to deck
            </Link>
          </div>
        )}

        {/* Review session */}
        {!loading && !error && !done && cards.length > 0 && (
          <div className="space-y-4 animate-slide-up">
            {/* Progress */}
            <div className="flex items-center justify-between text-[11px] text-text-dim">
              <span>
                Card {currentIndex + 1} of {cards.length}
              </span>
              <div className="flex-1 mx-3 h-1 bg-border-dim rounded-full overflow-hidden">
                <div
                  className="h-full rounded-full bg-neon-blue transition-all duration-500"
                  style={{ width: `${((currentIndex + 1) / cards.length) * 100}%` }}
                />
              </div>
              <span className="font-mono">
                {Math.round(((currentIndex + 1) / cards.length) * 100)}%
              </span>
            </div>

            {/* Rate error (non-blocking) */}
            {rateError && (
              <div className="text-[10px] text-red-400 bg-red-500/10 border border-red-500/20 rounded px-3 py-2">
                Review sync failed: {rateError} — progress saved locally.
              </div>
            )}

            {/* Flashcard */}
            <FlashCard
              card={cards[currentIndex]}
              isRevealed={isRevealed}
              onReveal={handleReveal}
              onRate={handleRate}
            />
          </div>
        )}
      </div>
    </div>
  )
}
