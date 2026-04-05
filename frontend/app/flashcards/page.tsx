'use client'

import { useEffect, useState, useCallback } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import DeckSummary from '@/components/flashcard/DeckSummary'
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

// ── Helpers ──────────────────────────────────────────────────────────────────

/**
 * Formats an ISO timestamp as a short locale date string.
 */
function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

/**
 * Truncates text to at most `maxLen` characters, appending an ellipsis if
 * the original string is longer.
 */
function truncate(text: string, maxLen = 80): string {
  return text.length > maxLen ? text.slice(0, maxLen) + '…' : text
}

// ── Loading skeleton ─────────────────────────────────────────────────────────

/** Skeleton placeholder shown while the deck is loading. */
function LoadingSkeleton() {
  return (
    <div className="space-y-4 animate-pulse">
      <div className="bg-bg-card border border-border-dim rounded-xl p-6">
        <div className="flex justify-around">
          <div className="flex flex-col items-center gap-2">
            <div className="h-8 w-12 bg-bg-panel rounded" />
            <div className="h-2.5 w-16 bg-bg-panel rounded" />
          </div>
          <div className="h-10 w-px bg-border-dim" />
          <div className="flex flex-col items-center gap-2">
            <div className="h-8 w-12 bg-bg-panel rounded" />
            <div className="h-2.5 w-16 bg-bg-panel rounded" />
          </div>
        </div>
        <div className="mt-5 space-y-2">
          <div className="h-8 w-full bg-bg-panel rounded-xl" />
          <div className="h-8 w-full bg-bg-panel rounded-xl" />
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="bg-bg-card border border-border-dim rounded-xl p-4">
            <div className="h-3.5 w-3/4 bg-bg-panel rounded mb-2" />
            <div className="h-2.5 w-1/3 bg-bg-panel rounded" />
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Card grid item ───────────────────────────────────────────────────────────

/** Renders a single flashcard summary tile in the deck grid. */
function CardTile({ card }: { card: Flashcard }) {
  return (
    <div className="bg-bg-card border border-border-mid rounded-xl p-4 hover:border-neon-blue/30 transition-colors">
      <p className="text-xs text-text-bright leading-relaxed mb-2">{truncate(card.front)}</p>
      <div className="flex items-center gap-2 flex-wrap">
        {card.topic && (
          <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-neon-blue/10 text-neon-blue border border-neon-blue/20">
            {card.topic}
          </span>
        )}
        <span className="text-[10px] text-text-dim">
          Interval: {card.interval_days}d
        </span>
        <span className="text-[10px] text-text-dim ml-auto">
          Next: {formatDate(card.next_review_at)}
        </span>
      </div>
    </div>
  )
}

// ── Main page ────────────────────────────────────────────────────────────────

/** Response shape from GET /api/flashcards */
interface FlashcardsResponse {
  cards: Flashcard[]
  due_count: number
  total: number
}

/**
 * Flashcard dashboard page (/flashcards).
 *
 * Loads the user's full deck on mount, shows a DeckSummary with actions
 * (review + Anki export), and renders all cards in a grid below.
 */
export default function FlashcardsPage() {
  const router = useRouter()
  const [cards, setCards] = useState<Flashcard[]>([])
  const [dueCount, setDueCount] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  /** Fetches all cards from the API proxy. */
  const loadDeck = useCallback(async () => {
    try {
      const res = await fetch('/api/flashcards', { cache: 'no-store' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data: FlashcardsResponse = await res.json()
      setCards(data.cards ?? [])
      setDueCount(data.due_count ?? 0)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load flashcards')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadDeck()
  }, [loadDeck])

  /**
   * Fetches the binary .apkg file from the Anki export endpoint and triggers
   * a browser download using a temporary <a> element.
   */
  const handleExportAnki = useCallback(async () => {
    try {
      const res = await fetch('/api/flashcards/export/anki')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const blob = await res.blob()
      const disposition = res.headers.get('content-disposition') ?? ''
      const nameMatch = disposition.match(/filename="?([^";]+)"?/)
      const filename = nameMatch?.[1] ?? 'flashcards.apkg'
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    } catch {
      // Non-fatal — just log; user sees nothing downloaded
      console.error('Anki export failed')
    }
  }, [])

  return (
    <div className="min-h-screen bg-bg-deep px-4 py-8 flex flex-col items-center">
      <div className="w-full max-w-2xl">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <Link href="/" className="text-xs text-text-dim hover:text-neon-blue transition-colors">
            ← Dashboard
          </Link>
          <h1 className="text-sm font-semibold text-text-bright tracking-wide uppercase">
            Flashcards
          </h1>
          <div className="w-16" />
        </div>

        {/* Body */}
        {loading && <LoadingSkeleton />}

        {error && (
          <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-4 py-3 mb-4">
            {error}
          </div>
        )}

        {!loading && !error && (
          <div className="space-y-6 animate-fade-in">
            {/* Deck summary */}
            <DeckSummary
              total={cards.length}
              dueCount={dueCount}
              onStartReview={() => router.push('/flashcards/review')}
              onExportAnki={handleExportAnki}
            />

            {/* Card grid */}
            {cards.length === 0 ? (
              <div className="bg-bg-card border border-border-dim rounded-xl p-8 text-center">
                <div className="text-3xl mb-3">🃏</div>
                <p className="text-sm text-text-dim leading-relaxed">
                  No flashcards yet — complete a quiz to generate your first cards
                </p>
              </div>
            ) : (
              <div>
                <h2 className="text-xs text-text-dim uppercase tracking-wider mb-3">
                  All cards ({cards.length})
                </h2>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  {cards.map((card) => (
                    <CardTile key={card.id} card={card} />
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
